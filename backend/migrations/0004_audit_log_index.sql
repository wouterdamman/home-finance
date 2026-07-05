-- +goose Up
CREATE INDEX audit_log_created_at_idx ON audit_log (created_at DESC);

-- +goose Down
DROP INDEX audit_log_created_at_idx;
