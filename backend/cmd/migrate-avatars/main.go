// migrate-avatars copies avatar files from the filesystem backend
// (avatar_object_key on disk) into the Postgres bytea backend
// (avatar_data), so the avatarStorage.filesystem PVC/volumeMount can be
// retired. Run this inside the pod that still has the PVC mounted, before
// deploying a release that removes the PVC — see deploy/helm/home-finance/README.md.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	dsn := flag.String("dsn", os.Getenv("DATABASE_URL"), "postgres DSN")
	dryRun := flag.Bool("dry-run", false, "list what would be migrated without writing")
	flag.Parse()

	if *dsn == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL or --dsn required")
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, *dsn)
	if err != nil {
		slog.Error("connect", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		slog.Error("ping", "err", err)
		os.Exit(1)
	}

	rows, err := pool.Query(ctx, `SELECT id, avatar_object_key, avatar_content_type FROM users WHERE avatar_object_key IS NOT NULL AND avatar_data IS NULL`)
	if err != nil {
		slog.Error("query", "err", err)
		os.Exit(1)
	}
	type candidate struct {
		id          int64
		objectKey   string
		contentType *string
	}
	var candidates []candidate
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.id, &c.objectKey, &c.contentType); err != nil {
			slog.Error("scan", "err", err)
			os.Exit(1)
		}
		candidates = append(candidates, c)
	}
	if err := rows.Err(); err != nil {
		slog.Error("iterate", "err", err)
		os.Exit(1)
	}

	fmt.Printf("found %d avatar(s) to migrate\n", len(candidates))
	migrated, failed := 0, 0
	for _, c := range candidates {
		data, err := os.ReadFile(c.objectKey)
		if err != nil {
			slog.Error("read file", "user_id", c.id, "path", c.objectKey, "err", err)
			failed++
			continue
		}
		if *dryRun {
			fmt.Printf("[dry-run] user %d: would copy %s (%d bytes, content-type %v)\n", c.id, c.objectKey, len(data), c.contentType)
			migrated++
			continue
		}
		if _, err := pool.Exec(ctx, `UPDATE users SET avatar_data=$2 WHERE id=$1`, c.id, data); err != nil {
			slog.Error("update", "user_id", c.id, "err", err)
			failed++
			continue
		}
		fmt.Printf("user %d: migrated %s (%d bytes)\n", c.id, c.objectKey, len(data))
		migrated++
	}

	fmt.Printf("\ndone: %d migrated, %d failed\n", migrated, failed)
	if failed > 0 {
		os.Exit(1)
	}
}
