// Package s3client builds the shared S3-compatible object storage client
// used by both avatar upload/proxy (internal/httpapi) and the background
// audit-log exporter (internal/auditexport) — one S3Config, one client
// constructor, so both features are configured (or left disabled) together.
package s3client

import (
	"log/slog"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/wouterdamman/home-finance/internal/config"
)

// New returns nil (not an error) when S3 isn't configured — callers check
// for that and disable the S3-dependent feature instead of failing startup.
func New(cfg *config.Config) *minio.Client {
	if !cfg.S3Configured() {
		return nil
	}
	client, err := minio.New(cfg.S3Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.S3AccessKey, cfg.S3SecretKey, ""),
		Secure: cfg.S3UseSSL,
		Region: cfg.S3Region,
	})
	if err != nil {
		slog.Error("s3 client init failed", "err", err)
		return nil
	}
	return client
}
