package acceptance_test

import (
	"testing"

	"task200-fiberorient/internal/model"
	"task200-fiberorient/internal/result"
)

func TestResultComputeFreezeAndVersioning(t *testing.T) {
	app := newAcceptanceApp(t)
	if _, err := app.Batches.Create("batch-result", "results", "paper", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Batches.Transition("batch-result", model.BatchObserving); err != nil {
		t.Fatal(err)
	}
	for _, field := range []struct {
		id    string
		slice float64
		data  []float64
	}{{"field-result-0", 0, []float64{30, 31, 30, 29, 30}}, {"field-result-90", 90, []float64{120, 121, 120, 119, 120}}} {
		if _, err := app.Fields.RegisterWithID(field.id, "batch-result", field.id, field.slice); err != nil {
			t.Fatal(err)
		}
		if _, err := app.Obs.Import("batch-result", field.id, model.UnitDegrees, field.data, field.id+"-submission"); err != nil {
			t.Fatal(err)
		}
		if _, err := app.Fields.MarkValid(field.id); err != nil {
			t.Fatal(err)
		}
	}
	draft, err := app.Cals.EstimateAndDraft("batch-result", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Cals.Activate(draft.Calibration.ID); err != nil {
		t.Fatal(err)
	}

	first, err := app.Results.Compute(result.ComputeOptions{
		BatchID:       "batch-result",
		CalibrationID: draft.Calibration.ID,
		FieldIDs:      []string{"field-result-0", "field-result-90"},
		CiLevel:       0.95,
		Seed:          7,
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.Version != 1 {
		t.Fatalf("first result version = %d", first.Version)
	}
	if _, err := app.Results.Freeze(first.ID); err != nil {
		t.Fatal(err)
	}
	second, err := app.Results.Compute(result.ComputeOptions{
		BatchID:       "batch-result",
		CalibrationID: draft.Calibration.ID,
		FieldIDs:      []string{"field-result-0", "field-result-90"},
		CiLevel:       0.95,
		Seed:          8,
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.Version != 2 {
		t.Fatalf("second result version = %d", second.Version)
	}
	comparison, err := app.Results.Compare(first.ID, second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if comparison.CountDelta != 0 {
		t.Fatalf("count delta = %d, want 0", comparison.CountDelta)
	}
}
