package acceptance_test

import (
	"testing"

	"task200-fiberorient/internal/model"
	"task200-fiberorient/internal/result"
)

func TestBug10_PublishRequiresAnalyzableBatch(t *testing.T) {
	app := newAcceptanceApp(t)
	if _, err := app.Batches.Create("batch-bug10", "publish", "paper", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Batches.Transition("batch-bug10", model.BatchObserving); err != nil {
		t.Fatal(err)
	}
	for _, field := range []struct {
		id    string
		slice float64
		angle float64
	}{{"bug10-field-0", 0, 30}, {"bug10-field-90", 90, 120}} {
		if _, err := app.Fields.RegisterWithID(field.id, "batch-bug10", field.id, field.slice); err != nil {
			t.Fatal(err)
		}
		if _, err := app.Obs.Import("batch-bug10", field.id, model.UnitDegrees, []float64{field.angle, field.angle, field.angle, field.angle, field.angle}, field.id+"-submission"); err != nil {
			t.Fatal(err)
		}
		if _, err := app.Fields.MarkValid(field.id); err != nil {
			t.Fatal(err)
		}
	}
	draft, err := app.Cals.EstimateAndDraft("batch-bug10", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Cals.Activate(draft.Calibration.ID); err != nil {
		t.Fatal(err)
	}
	computed, err := app.Results.Compute(result.ComputeOptions{BatchID: "batch-bug10", CalibrationID: draft.Calibration.ID, FieldIDs: []string{"bug10-field-0", "bug10-field-90"}, CiLevel: 0.95, Seed: 10})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Results.Freeze(computed.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Publish("batch-bug10"); err == nil {
		t.Fatal("observing batch was published without analyzable state")
	}
	batch, err := app.Batches.Get("batch-bug10")
	if err != nil {
		t.Fatal(err)
	}
	if batch.Status != model.BatchObserving {
		t.Fatalf("batch status = %s, want observing", batch.Status)
	}
}
