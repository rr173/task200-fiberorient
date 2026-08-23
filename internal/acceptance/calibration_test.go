package acceptance_test

import (
	"testing"

	"task200-fiberorient/internal/model"
)

func TestCalibrationDraftActivationAndLookup(t *testing.T) {
	app := newAcceptanceApp(t)
	if _, err := app.Batches.Create("batch-cal", "calibration", "paper", 45); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Batches.Transition("batch-cal", model.BatchObserving); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Fields.RegisterWithID("field-cal-0", "batch-cal", "zero", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Fields.RegisterWithID("field-cal-90", "batch-cal", "ninety", 90); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Obs.Import("batch-cal", "field-cal-0", model.UnitDegrees, []float64{40, 41, 42}, "sub-0"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Obs.Import("batch-cal", "field-cal-90", model.UnitDegrees, []float64{108, 109, 110}, "sub-90"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Fields.MarkValid("field-cal-0"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Fields.MarkValid("field-cal-90"); err != nil {
		t.Fatal(err)
	}

	draft, err := app.Cals.EstimateAndDraft("batch-cal", 0)
	if err != nil {
		t.Fatal(err)
	}
	if draft.SampleSize != 6 || draft.Calibration.Status != model.CalibrationDraft {
		t.Fatalf("unexpected draft: sample=%d status=%s", draft.SampleSize, draft.Calibration.Status)
	}
	active, err := app.Cals.Activate(draft.Calibration.ID)
	if err != nil {
		t.Fatal(err)
	}
	if active.Status != model.CalibrationActive {
		t.Fatalf("activated calibration status = %s", active.Status)
	}
	loaded, err := app.Cals.Active("batch-cal")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ID != active.ID {
		t.Fatalf("active calibration id = %s, want %s", loaded.ID, active.ID)
	}
}
