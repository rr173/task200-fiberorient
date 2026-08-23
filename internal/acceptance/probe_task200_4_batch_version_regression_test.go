package acceptance_test

import (
	"testing"

	"task200-fiberorient/internal/model"
	"task200-fiberorient/internal/store"
)

func TestBug04_BatchVersionRejectsStaleUpdate(t *testing.T) {
	app := newAcceptanceApp(t)
	if _, err := app.Batches.Create("batch-bug4", "version", "paper", 0); err != nil {
		t.Fatal(err)
	}
	stale, err := app.Batches.Get("batch-bug4")
	if err != nil {
		t.Fatal(err)
	}
	updated, err := app.Batches.UpdateSliceAngle("batch-bug4", 45)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Version != 2 {
		t.Fatalf("updated version = %d, want 2", updated.Version)
	}
	stale.Name = "stale write"
	stale.Status = model.BatchObserving
	if err := store.NewBatchStore(app.DB()).Update(stale); err == nil {
		t.Fatal("stale update unexpectedly succeeded")
	}
}
