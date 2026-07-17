// Package auditexport periodically ships new audit_log rows to S3-compatible
// object storage as newline-delimited JSON. It's entirely opt-in: disabled
// whenever S3 isn't configured or AUDIT_EXPORT_INTERVAL is unset, and a
// failed export just logs a warning and retries next tick — it must never
// take down the app or block a user-facing request (matches the existing
// best-effort philosophy of s.auditLog() itself).
package auditexport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"

	"github.com/wouterdamman/home-finance/internal/config"
	"github.com/wouterdamman/home-finance/internal/s3client"
)

// batchSize caps how many rows one export tick ships — a short interval
// (e.g. 10s) combined with a burst of activity shouldn't build an
// unbounded single upload; the next tick just picks up where this one
// left off.
const batchSize = 5000

type entry struct {
	ID         int64           `json:"id"`
	CreatedAt  time.Time       `json:"createdAt"`
	UserEmail  *string         `json:"userEmail,omitempty"`
	Action     string          `json:"action"`
	EntityType *string         `json:"entityType,omitempty"`
	EntityID   *int64          `json:"entityId,omitempty"`
	Details    json.RawMessage `json:"details,omitempty"`
}

// Start launches the background export loop and returns immediately; it
// no-ops (logs once, does nothing further) when S3 or the interval isn't
// configured. Call it in its own goroutine — it blocks until ctx is done.
func Start(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config) {
	if cfg.AuditExportInterval == "" {
		return
	}
	interval, err := time.ParseDuration(cfg.AuditExportInterval)
	if err != nil {
		slog.Error("auditexport: invalid AUDIT_EXPORT_INTERVAL", "value", cfg.AuditExportInterval, "err", err)
		return
	}
	client := s3client.New(cfg)
	if client == nil {
		slog.Warn("auditexport: AUDIT_EXPORT_INTERVAL is set but S3 is not configured — export disabled")
		return
	}

	slog.Info("auditexport: starting", "interval", interval, "bucket", cfg.S3Bucket)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := exportOnce(ctx, pool, client, cfg.S3Bucket); err != nil {
				slog.Warn("auditexport: export failed, will retry next tick", "err", err)
			}
		}
	}
}

func exportOnce(ctx context.Context, pool *pgxpool.Pool, client *minio.Client, bucket string) error {
	var lastExported int64
	if err := pool.QueryRow(ctx, `SELECT last_exported_id FROM audit_log_export_state WHERE id=1`).Scan(&lastExported); err != nil {
		return fmt.Errorf("read export state: %w", err)
	}

	rows, err := pool.Query(ctx, `
		SELECT id, created_at, user_email, action, entity_type, entity_id, details
		FROM audit_log WHERE id > $1 ORDER BY id ASC LIMIT $2`, lastExported, batchSize)
	if err != nil {
		return fmt.Errorf("query audit_log: %w", err)
	}
	defer rows.Close()

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	maxID := lastExported
	count := 0
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.ID, &e.CreatedAt, &e.UserEmail, &e.Action, &e.EntityType, &e.EntityID, &e.Details); err != nil {
			return fmt.Errorf("scan audit_log row: %w", err)
		}
		if err := enc.Encode(e); err != nil {
			return fmt.Errorf("encode audit_log row: %w", err)
		}
		maxID = e.ID
		count++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate audit_log rows: %w", err)
	}
	if count == 0 {
		return nil
	}

	now := time.Now().UTC()
	key := fmt.Sprintf("audit-log/%04d/%02d/audit-%d.jsonl", now.Year(), now.Month(), now.UnixNano())
	if _, err := client.PutObject(ctx, bucket, key, &buf, int64(buf.Len()), minio.PutObjectOptions{ContentType: "application/x-ndjson"}); err != nil {
		return fmt.Errorf("upload to s3: %w", err)
	}

	if _, err := pool.Exec(ctx, `UPDATE audit_log_export_state SET last_exported_id=$1, updated_at=now() WHERE id=1`, maxID); err != nil {
		return fmt.Errorf("update export state: %w", err)
	}
	slog.Info("auditexport: exported", "count", count, "key", key)
	return nil
}
