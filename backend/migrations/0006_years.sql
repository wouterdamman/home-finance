-- +goose Up
CREATE TABLE years (
    year       INT PRIMARY KEY CHECK (year BETWEEN 2000 AND 2100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO years (year) SELECT DISTINCT year FROM periods ON CONFLICT DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS years;
