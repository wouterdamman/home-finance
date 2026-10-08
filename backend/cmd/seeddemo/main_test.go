package main

import "testing"

func TestRequireLocalHost(t *testing.T) {
	ok := []string{
		"postgres://homefinance:homefinance@localhost:5433/homefinance?sslmode=disable",
		"postgres://u:p@127.0.0.1:5432/db",
		"host=localhost port=5432 dbname=db",
	}
	bad := []string{
		"postgres://u:p@localhost/db?host=prod-db.internal",
		"postgres://u:p@prod-db.internal/db",
		"host=prod.internal port=5432 dbname=db",
		"postgres://u:p@localhost/db?host=localhost,prod.internal",
	}
	for _, dsn := range ok {
		if err := requireLocalHost(dsn); err != nil {
			t.Errorf("expected accept %q, got %v", dsn, err)
		}
	}
	for _, dsn := range bad {
		if err := requireLocalHost(dsn); err == nil {
			t.Errorf("expected reject %q", dsn)
		}
	}
}
