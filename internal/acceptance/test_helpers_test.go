package acceptance_test

import (
	"testing"

	"task200-fiberorient/internal/service"
	"task200-fiberorient/internal/store"
)

func newAcceptanceApp(t *testing.T) *service.App {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	app, err := service.New(db)
	if err != nil {
		t.Fatalf("construct application: %v", err)
	}
	return app
}
