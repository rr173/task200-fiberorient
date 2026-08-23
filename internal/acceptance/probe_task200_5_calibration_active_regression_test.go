package acceptance_test

import (
	"testing"

	"task200-fiberorient/internal/model"
	"task200-fiberorient/internal/store"
)

func TestBug05_ActivatingCalibrationRevokesPreviousActive(t *testing.T) {
	app := newAcceptanceApp(t)
	if _, err := app.Batches.Create("batch-bug5", "calibration", "paper", 0); err != nil {
		t.Fatal(err)
	}
	cs := store.NewCalibrationStore(app.DB())
	first, err := model.NewCalibration("bug5-cal-1", "batch-bug5", 12, 6, 0.9)
	if err != nil {
		t.Fatal(err)
	}
	second, err := model.NewCalibration("bug5-cal-2", "batch-bug5", 13, 6, 0.9)
	if err != nil {
		t.Fatal(err)
	}
	if err := cs.Insert(first); err != nil {
		t.Fatal(err)
	}
	if err := cs.Insert(second); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Cals.Activate(first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Cals.Activate(second.ID); err != nil {
		t.Fatal(err)
	}
	all, err := app.Cals.ListByBatch("batch-bug5")
	if err != nil {
		t.Fatal(err)
	}
	active := 0
	for _, calibration := range all {
		if calibration.Status == model.CalibrationActive {
			active++
		}
	}
	if active != 1 {
		t.Fatalf("active calibration count = %d, want 1", active)
	}
}
