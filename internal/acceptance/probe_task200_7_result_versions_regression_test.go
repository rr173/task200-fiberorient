package acceptance_test

import (
	"sync"
	"testing"

	"task200-fiberorient/internal/model"
	"task200-fiberorient/internal/result"
)

func TestBug07_ConcurrentComputesAllocateDistinctVersions(t *testing.T) {
	app := newAcceptanceApp(t)
	if _, err := app.Batches.Create("batch-bug7", "versions", "paper", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Batches.Transition("batch-bug7", model.BatchObserving); err != nil {
		t.Fatal(err)
	}
	for _, field := range []struct {
		id    string
		slice float64
		angle float64
	}{{"bug7-field-0", 0, 30}, {"bug7-field-90", 90, 120}} {
		if _, err := app.Fields.RegisterWithID(field.id, "batch-bug7", field.id, field.slice); err != nil {
			t.Fatal(err)
		}
		if _, err := app.Obs.Import("batch-bug7", field.id, model.UnitDegrees, []float64{field.angle, field.angle, field.angle, field.angle, field.angle}, field.id+"-submission"); err != nil {
			t.Fatal(err)
		}
		if _, err := app.Fields.MarkValid(field.id); err != nil {
			t.Fatal(err)
		}
	}
	draft, err := app.Cals.EstimateAndDraft("batch-bug7", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Cals.Activate(draft.Calibration.ID); err != nil {
		t.Fatal(err)
	}
	const workers = 20
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(seed int64) {
			defer wg.Done()
			_, err := app.Results.Compute(result.ComputeOptions{BatchID: "batch-bug7", CalibrationID: draft.Calibration.ID, FieldIDs: []string{"bug7-field-0", "bug7-field-90"}, CiLevel: 0.95, Seed: seed})
			if err != nil {
				errs <- err
			}
		}(int64(i + 1))
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent compute failed: %v", err)
	}
	results, err := app.Results.ListByBatch("batch-bug7")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != workers {
		t.Fatalf("result count = %d, want %d", len(results), workers)
	}
	for i, item := range results {
		want := workers - i
		if item.Version != want {
			t.Fatalf("result[%d] version = %d, want %d", i, item.Version, want)
		}
	}
}
