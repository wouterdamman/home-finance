-- +goose Up

CREATE TABLE category_description_presets (
  id BIGSERIAL PRIMARY KEY,
  category_id BIGINT NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
  description TEXT NOT NULL,
  sort_order INT NOT NULL DEFAULT 0,
  UNIQUE(category_id, description)
);

CREATE TABLE income_source_description_presets (
  id BIGSERIAL PRIMARY KEY,
  income_source_id BIGINT NOT NULL REFERENCES income_sources(id) ON DELETE CASCADE,
  description TEXT NOT NULL,
  sort_order INT NOT NULL DEFAULT 0,
  UNIQUE(income_source_id, description)
);

-- +goose Down

DROP TABLE income_source_description_presets;
DROP TABLE category_description_presets;
