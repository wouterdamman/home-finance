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
	"net"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/encrypt"

	"github.com/wouterdamman/home-finance/internal/config"
	"github.com/wouterdamman/home-finance/internal/s3client"
)

// batchSize caps how many rows one export tick ships — a short interval
// (e.g. 10s) combined with a burst of activity shouldn't build an
// unbounded single upload; the next tick just picks up where this one
// left off.
const batchSize = 5000

// visibilityLag keeps the cursor behind rows that may still be in flight.
// audit_log ids are allocated at INSERT but only become visible at COMMIT, so
// a row written by a still-open transaction would otherwise be stepped over
// permanently — a silent hole in the off-box copy of the trail.
const visibilityLag = "1 minute"

// Per-tick deadline. minio-go's default transport has no response timeout, so
// an endpoint that accepts the connection and then stalls would block
// PutObject forever and the loop would never return to its select.
const (
	minTickTimeout = 30 * time.Second
	maxTickTimeout = 5 * time.Minute
)

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
//
// Every failure path here returns rather than panicking: this runs as a bare
// goroutine, so middleware.Recoverer does not cover it and a panic would take
// the whole pod down.
func Start(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config) {
	if cfg.AuditExportInterval == "" {
		return
	}
	interval, err := time.ParseDuration(cfg.AuditExportInterval)
	if err != nil {
		slog.Error("auditexport: invalid AUDIT_EXPORT_INTERVAL", "value", cfg.AuditExportInterval, "err", err)
		return
	}
	// "0" and "-5s" both parse fine but panic time.NewTicker. "0" meaning
	// "off" is a plausible operator typo in the Helm value, which is free text.
	if interval <= 0 {
		slog.Error("auditexport: AUDIT_EXPORT_INTERVAL must be a positive duration — export disabled",
			"value", cfg.AuditExportInterval)
		return
	}
	if !transportIsSafe(cfg) {
		slog.Error("auditexport: refusing to start — S3_USE_SSL is false and the endpoint is not loopback; "+
			"every family member's email and financial action would be shipped in cleartext alongside static V4 credentials",
			"endpoint", cfg.S3Endpoint)
		return
	}
	client := s3client.New(cfg)
	if client == nil {
		slog.Warn("auditexport: AUDIT_EXPORT_INTERVAL is set but S3 is not configured — export disabled")
		return
	}

	tickTimeout := interval * 3 / 4
	if tickTimeout < minTickTimeout {
		tickTimeout = minTickTimeout
	}
	if tickTimeout > maxTickTimeout {
		tickTimeout = maxTickTimeout
	}

	slog.Info("auditexport: starting", "interval", interval, "bucket", cfg.S3Bucket, "tickTimeout", tickTimeout)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("auditexport: stopping")
			return
		case <-ticker.C:
			tickCtx, cancel := context.WithTimeout(ctx, tickTimeout)
			err := exportOnce(tickCtx, pool, client, cfg.S3Bucket)
			cancel()
			if err != nil && ctx.Err() == nil {
				slog.Warn("auditexport: export failed, will retry next tick", "err", err)
			}
		}
	}
}

// transportIsSafe reports whether the audit stream would leave the pod
// encrypted. A plaintext endpoint is only tolerated for a loopback sidecar.
func transportIsSafe(cfg *config.Config) bool {
	if cfg.S3UseSSL {
		return true
	}
	host := cfg.S3Endpoint
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func exportOnce(ctx context.Context, pool *pgxpool.Pool, client *minio.Client, bucket string) error {
	var lastExported int64
	if err := pool.QueryRow(ctx, `SELECT last_exported_id FROM audit_log_export_state WHERE id=1`).Scan(&lastExported); err != nil {
		return fmt.Errorf("read export state: %w", err)
	}

	rows, err := pool.Query(ctx, `
		SELECT id, created_at, user_email, action, entity_type, entity_id, details
		FROM audit_log
		WHERE id > $1 AND created_at < now() - interval '`+visibilityLag+`'
		ORDER BY id ASC LIMIT $2`, lastExported, batchSize)
	if err != nil {
		return fmt.Errorf("query audit_log: %w", err)
	}
	defer rows.Close()

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	minID := int64(0)
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
		if count == 0 {
			minID = e.ID
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
	// The id range lives in the object name as well as in the state row, so a
	// gap or a duplicated range is detectable from the off-box copy alone —
	// which is the point of having a copy the DB can't be trusted to describe.
	key := fmt.Sprintf("audit-log/%04d/%02d/audit-%012d-%012d-%d.jsonl", now.Year(), now.Month(), minID, maxID, now.UnixNano())
	opts := minio.PutObjectOptions{
		ContentType: "application/x-ndjson",
		// Each object carries every family member's email next to every
		// financial action; ask the bucket to encrypt it at rest.
		ServerSideEncryption: encrypt.NewSSE(),
	}
	if _, err := client.PutObject(ctx, bucket, key, &buf, int64(buf.Len()), opts); err != nil {
		return fmt.Errorf("upload to s3: %w", err)
	}

	if _, err := pool.Exec(ctx, `
		UPDATE audit_log_export_state
		SET last_exported_id=$1, last_export_first_id=$2, last_export_last_id=$1, last_export_key=$3, updated_at=now()
		WHERE id=1`, maxID, minID, key); err != nil {
		return fmt.Errorf("update export state: %w", err)
	}
	slog.Info("auditexport: exported", "count", count, "firstId", minID, "lastId", maxID, "key", key)
	return nil
}
