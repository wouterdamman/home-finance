// Package s3client builds the shared S3-compatible object storage client
// used by the background audit-log exporter (internal/auditexport) — one
// S3Config, one client constructor, so the S3-dependent features are
// configured (or left disabled) together.
package s3client

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/wouterdamman/home-finance/internal/config"
)

// responseHeaderTimeout bounds the wait for response headers after the request
// body is written. minio-go's transport sets none, so an endpoint that accepts
// the connection and then stalls would block a PutObject indefinitely.
const responseHeaderTimeout = 60 * time.Second

// New returns nil (not an error) when S3 isn't configured — callers check
// for that and disable the S3-dependent feature instead of failing startup.
func New(cfg *config.Config) *minio.Client {
	if !cfg.S3Configured() {
		return nil
	}
	if !cfg.S3UseSSL {
		// Static V4 credentials and the object payload both travel in the clear.
		// Callers that ship sensitive data (auditexport) refuse to start on this
		// rather than relying on the log being read.
		slog.Warn("S3_USE_SSL=false — S3 credentials and object contents are sent unencrypted; only safe for a loopback endpoint",
			"endpoint", cfg.S3Endpoint)
	}

	var transport http.RoundTripper
	if t, err := minio.DefaultTransport(cfg.S3UseSSL); err != nil {
		slog.Warn("s3 client: falling back to default transport", "err", err)
	} else {
		t.ResponseHeaderTimeout = responseHeaderTimeout
		transport = t
	}

	client, err := minio.New(cfg.S3Endpoint, &minio.Options{
		Creds:     credentials.NewStaticV4(cfg.S3AccessKey, cfg.S3SecretKey, ""),
		Secure:    cfg.S3UseSSL,
		Region:    cfg.S3Region,
		Transport: transport,
	})
	if err != nil {
		slog.Error("s3 client init failed", "err", err)
		return nil
	}
	return client
}
