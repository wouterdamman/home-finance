-- +goose Up

ALTER TABLE categories ADD COLUMN parent_id BIGINT REFERENCES categories(id);
CREATE INDEX categories_parent_id_idx ON categories (parent_id);

-- Purely additive: parent_id starts NULL on every existing row, so this
-- view degenerates to member_id = category_id for all of them until a
-- child category actually exists — identical behavior to before this
-- migration. Every aggregation query that used to filter/group on
-- t.category_id = bl.category_id joins through this view instead, so a
-- child category's transactions roll up into its parent's budget line.
CREATE VIEW category_rollup AS
SELECT id AS category_id, id AS member_id FROM categories
UNION ALL
SELECT parent_id AS category_id, id AS member_id FROM categories WHERE parent_id IS NOT NULL;

-- Maps an import Details-table header (e.g. "Bunq") to the parent category
-- it should be nested under (e.g. "Boodschappen") instead of becoming its
-- own disconnected top-level category. Consulted only the first time a
-- given header has no existing categories.name match at all.
CREATE TABLE category_aliases (
    id                 BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    alias_name         TEXT NOT NULL UNIQUE,
    parent_category_id BIGINT NOT NULL REFERENCES categories(id) ON DELETE CASCADE
);

-- +goose Down

DROP TABLE category_aliases;
DROP VIEW category_rollup;
DROP INDEX categories_parent_id_idx;
ALTER TABLE categories DROP COLUMN parent_id;
