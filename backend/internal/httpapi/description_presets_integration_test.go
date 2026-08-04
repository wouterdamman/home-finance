//go:build integration

package httpapi_test

import (
	"context"
	"net/http"
	"strconv"
	"testing"
)

// TestCategoryDescriptionPresetsCRUD covers the #22 curated-suggestion-list
// feature: presets are managed separately from the auto-derived
// transaction-descriptions history, and cascade-delete when their category
// is removed.
func TestCategoryDescriptionPresetsCRUD(t *testing.T) {
	srv, pool := newIntegrationServer(t)
	c := newAPIClient(t, srv)
	ctx := context.Background()

	const name = "ZTest Presets Category 2099"
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM categories WHERE name=$1`, name)
	})

	resp := c.do(http.MethodPost, "/api/categories", map[string]any{
		"name": name, "defaultAmountCents": 0, "isItemized": true, "includeInTemplate": true, "sortOrder": 0,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create category: want 201, got %d", resp.StatusCode)
	}
	var cat struct {
		ID int64 `json:"id"`
	}
	c.decode(resp, &cat)
	catID := strconv.FormatInt(cat.ID, 10)

	resp = c.do(http.MethodPost, "/api/categories/"+catID+"/description-presets", map[string]any{"description": "Jumbo", "sortOrder": 0})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create preset: want 201, got %d", resp.StatusCode)
	}
	var preset struct {
		ID int64 `json:"id"`
	}
	c.decode(resp, &preset)

	// duplicate description for the same category is rejected
	resp = c.do(http.MethodPost, "/api/categories/"+catID+"/description-presets", map[string]any{"description": "Jumbo", "sortOrder": 1})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("create duplicate preset: want 409, got %d", resp.StatusCode)
	}

	resp = c.do(http.MethodGet, "/api/categories/"+catID+"/description-presets", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list presets: want 200, got %d", resp.StatusCode)
	}
	var list []struct {
		ID          int64  `json:"id"`
		Description string `json:"description"`
	}
	c.decode(resp, &list)
	if len(list) != 1 || list[0].Description != "Jumbo" {
		t.Fatalf("list presets: want [Jumbo], got %+v", list)
	}

	resp = c.do(http.MethodDelete, "/api/category-description-presets/"+strconv.FormatInt(preset.ID, 10), nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete preset: want 204, got %d", resp.StatusCode)
	}
	resp = c.do(http.MethodGet, "/api/categories/"+catID+"/description-presets", nil)
	c.decode(resp, &list)
	if len(list) != 0 {
		t.Fatalf("presets after delete: want empty, got %+v", list)
	}

	// cascade-delete: a preset on a category that then gets deleted should
	// disappear too, not orphan a row referencing a gone category_id
	resp = c.do(http.MethodPost, "/api/categories/"+catID+"/description-presets", map[string]any{"description": "Nettorama", "sortOrder": 0})
	c.decode(resp, &preset)
	if _, err := pool.Exec(ctx, `DELETE FROM categories WHERE id=$1`, cat.ID); err != nil {
		t.Fatalf("delete category: %v", err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM category_description_presets WHERE category_id=$1`, cat.ID).Scan(&count); err != nil {
		t.Fatalf("count orphaned presets: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected presets to cascade-delete with category, found %d remaining", count)
	}
}

// TestIncomeSourceDescriptionPresetsCRUD mirrors the category test for
// income sources, which are a separate parallel table by this codebase's
// existing convention (see transactions/income_transactions).
func TestIncomeSourceDescriptionPresetsCRUD(t *testing.T) {
	srv, pool := newIntegrationServer(t)
	c := newAPIClient(t, srv)
	ctx := context.Background()

	const name = "ZTest Presets Income Source 2099"
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM income_sources WHERE name=$1`, name)
	})

	resp := c.do(http.MethodPost, "/api/income-sources", map[string]any{
		"name": name, "defaultAmountCents": 0, "isItemized": true, "includeInTemplate": true, "sortOrder": 0,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create income source: want 201, got %d", resp.StatusCode)
	}
	var src struct {
		ID int64 `json:"id"`
	}
	c.decode(resp, &src)
	srcID := strconv.FormatInt(src.ID, 10)

	resp = c.do(http.MethodPost, "/api/income-sources/"+srcID+"/description-presets", map[string]any{"description": "Salaris", "sortOrder": 0})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create preset: want 201, got %d", resp.StatusCode)
	}
	var preset struct {
		ID int64 `json:"id"`
	}
	c.decode(resp, &preset)

	resp = c.do(http.MethodGet, "/api/income-sources/"+srcID+"/description-presets", nil)
	var list []struct {
		Description string `json:"description"`
	}
	c.decode(resp, &list)
	if len(list) != 1 || list[0].Description != "Salaris" {
		t.Fatalf("list presets: want [Salaris], got %+v", list)
	}

	resp = c.do(http.MethodDelete, "/api/income-source-description-presets/"+strconv.FormatInt(preset.ID, 10), nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete preset: want 204, got %d", resp.StatusCode)
	}
}
