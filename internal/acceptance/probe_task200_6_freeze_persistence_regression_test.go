package acceptance_test

import (
	"testing"

	"task200-fiberorient/internal/model"
	"task200-fiberorient/internal/result"
)

func TestBug06_FrozenResultRetainsFrozenTimestamp(t *testing.T) {
	app := newAcceptanceApp(t)
	if _, err := app.Batches.Create("batch-bug6", "freeze", "paper", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Batches.Transition("batch-bug6", model.BatchObserving); err != nil {
		t.Fatal(err)
	}
	for _, field := range []struct {
		id    string
		slice float64
		angle float64
	}{{"bug6-field-0", 0, 30}, {"bug6-field-90", 90, 120}} {
		if _, err := app.Fields.RegisterWithID(field.id, "batch-bug6", field.id, field.slice); err != nil {
			t.Fatal(err)
		}
		if _, err := app.Obs.Import("batch-bug6", field.id, model.UnitDegrees, []float64{field.angle, field.angle, field.angle, field.angle, field.angle}, field.id+"-submission"); err != nil {
			t.Fatal(err)
		}
		if _, err := app.Fields.MarkValid(field.id); err != nil {
			t.Fatal(err)
		}
	}
	draft, err := app.Cals.EstimateAndDraft("batch-bug6", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Cals.Activate(draft.Calibration.ID); err != nil {
		t.Fatal(err)
	}
	computed, err := app.Results.Compute(result.ComputeOptions{BatchID: "batch-bug6", CalibrationID: draft.Calibration.ID, FieldIDs: []string{"bug6-field-0", "bug6-field-90"}, CiLevel: 0.95, Seed: 6})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Results.Freeze(computed.ID); err != nil {
		t.Fatal(err)
	}
	reloaded, err := app.Results.Get(computed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Status != model.ResultFrozen || reloaded.FrozenAt == nil {
		t.Fatalf("reloaded result status=%s frozen_at=%v", reloaded.Status, reloaded.FrozenAt)
	}
}
