-- +goose Up
-- Singleton row tracking the last audit_log.id exported to S3, so the
-- background exporter (internal/auditexport) knows where to resume.
CREATE TABLE audit_log_export_state (
    id               BIGINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    last_exported_id BIGINT NOT NULL DEFAULT 0,
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO audit_log_export_state (id, last_exported_id) VALUES (1, 0);

-- +goose Down
DROP TABLE audit_log_export_state;
