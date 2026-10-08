//go:build integration

package httpapi_test

import (
	"context"
	"net/http"
	"testing"
)

func TestTrendsDashboardRoundTrip(t *testing.T) {
	srv, pool := newIntegrationServer(t)
	c := newAPIClient(t, srv)
	ctx := context.Background()

	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM trends_dashboards`) })
	pool.Exec(ctx, `DELETE FROM trends_dashboards`)

	var got struct {
		Widgets []map[string]any `json:"widgets"`
	}
	resp := c.do(http.MethodGet, "/api/me/trends-dashboard", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get empty: want 200, got %d", resp.StatusCode)
	}
	c.decode(resp, &got)
	if got.Widgets != nil {
		t.Fatalf("never-saved board must be null, got %v", got.Widgets)
	}

	put := map[string]any{"widgets": []map[string]any{
		{"id": "a", "visible": true, "width": 2, "height": 3, "config": map[string]any{"type": "sankeyFlow", "year": 2026}},
		{"id": "b", "visible": false, "width": 1, "height": 1, "config": map[string]any{"type": "kpi"}},
	}}
	resp = c.do(http.MethodPut, "/api/me/trends-dashboard", put)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("put: want 204, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Saving twice must replace, not append or conflict.
	put["widgets"] = put["widgets"].([]map[string]any)[:1]
	resp = c.do(http.MethodPut, "/api/me/trends-dashboard", put)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("second put: want 204, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = c.do(http.MethodGet, "/api/me/trends-dashboard", nil)
	c.decode(resp, &got)
	if len(got.Widgets) != 1 || got.Widgets[0]["id"] != "a" {
		t.Fatalf("want the one replaced widget, got %v", got.Widgets)
	}
}

func TestTrendsDashboardRejectsMalformed(t *testing.T) {
	srv, _ := newIntegrationServer(t)
	c := newAPIClient(t, srv)

	for name, body := range map[string]any{
		"missing widgets":  map[string]any{},
		"null widgets":     map[string]any{"widgets": nil},
		"non-object entry": map[string]any{"widgets": []any{1}},
		"null entry":       map[string]any{"widgets": []any{nil}},
	} {
		resp := c.do(http.MethodPut, "/api/me/trends-dashboard", body)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: want 400, got %d", name, resp.StatusCode)
		}
		resp.Body.Close()
	}

	tooMany := make([]map[string]any, 201)
	for i := range tooMany {
		tooMany[i] = map[string]any{"id": "x"}
	}
	resp := c.do(http.MethodPut, "/api/me/trends-dashboard", map[string]any{"widgets": tooMany})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("201 widgets: want 400, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}
