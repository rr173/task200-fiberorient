package acceptance_test

import (
	"testing"

	"task200-fiberorient/internal/model"
)

func TestBug01_SubmissionIdentityIncludesField(t *testing.T) {
	app := newAcceptanceApp(t)
	if _, err := app.Batches.Create("batch-bug1", "identity", "paper", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Fields.RegisterWithID("bug1-field-a", "batch-bug1", "a", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Fields.RegisterWithID("bug1-field-b", "batch-bug1", "b", 90); err != nil {
		t.Fatal(err)
	}
	first, err := app.Obs.Import("batch-bug1", "bug1-field-a", model.UnitDegrees, []float64{31}, "shared-submission")
	if err != nil {
		t.Fatal(err)
	}
	second, err := app.Obs.Import("batch-bug1", "bug1-field-b", model.UnitDegrees, []float64{109}, "shared-submission")
	if err != nil {
		t.Fatal(err)
	}
	if first.Inserted != 1 || second.Inserted != 1 {
		t.Fatalf("cross-field imports were not both inserted: first=%#v second=%#v", first, second)
	}
	count, err := app.Obs.CountByBatch("batch-bug1")
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("observation count = %d, want 2", count)
	}
}
