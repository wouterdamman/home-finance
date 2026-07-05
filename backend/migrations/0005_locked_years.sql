-- +goose Up
CREATE TABLE locked_years (
  year        INT         PRIMARY KEY,
  locked_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE locked_years;
