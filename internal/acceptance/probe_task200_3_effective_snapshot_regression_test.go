package acceptance_test

import (
	"testing"

	"task200-fiberorient/internal/model"
	"task200-fiberorient/internal/result"
)

func TestBug03_ExcludedFieldsStayOutOfResultSnapshot(t *testing.T) {
	app := newAcceptanceApp(t)
	if _, err := app.Batches.Create("batch-bug3", "snapshot", "paper", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Batches.Transition("batch-bug3", model.BatchObserving); err != nil {
		t.Fatal(err)
	}
	fields := []struct {
		id    string
		slice float64
		data  []float64
	}{{"bug3-good", 0, []float64{30, 30, 31, 30, 29}}, {"bug3-bad", 90, []float64{110, 110, 111, 109, 110}}}
	for _, field := range fields {
		if _, err := app.Fields.RegisterWithID(field.id, "batch-bug3", field.id, field.slice); err != nil {
			t.Fatal(err)
		}
		if _, err := app.Obs.Import("batch-bug3", field.id, model.UnitDegrees, field.data, field.id+"-submission"); err != nil {
			t.Fatal(err)
		}
		if _, err := app.Fields.MarkValid(field.id); err != nil {
			t.Fatal(err)
		}
	}
	draft, err := app.Cals.EstimateAndDraft("batch-bug3", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Cals.Activate(draft.Calibration.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Fields.MarkPolluted("bug3-bad", "dust"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Fields.Exclude("bug3-bad"); err != nil {
		t.Fatal(err)
	}
	fieldIDs, snapshot, err := app.FullStatSnapshot("batch-bug3")
	if err != nil {
		t.Fatal(err)
	}
	if len(fieldIDs) != 1 || fieldIDs[0] != "bug3-good" || snapshot != "bug3-good" {
		t.Fatalf("effective snapshot = ids=%v snapshot=%q", fieldIDs, snapshot)
	}
	computed, err := app.Results.Compute(result.ComputeOptions{BatchID: "batch-bug3", CalibrationID: draft.Calibration.ID, FieldIDs: fieldIDs, CiLevel: 0.95, Seed: 3})
	if err != nil {
		t.Fatal(err)
	}
	if computed.ObservationCount != 5 || computed.FieldSnapshot != "bug3-good" {
		t.Fatalf("result snapshot = %q count=%d", computed.FieldSnapshot, computed.ObservationCount)
	}
}
