package acceptance_test

import (
	"testing"

	"task200-fiberorient/internal/model"
)

func TestObservationImportIsIdempotentAndPreservesUnits(t *testing.T) {
	app := newAcceptanceApp(t)
	if _, err := app.Batches.Create("batch-obs", "observations", "paper", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Fields.RegisterWithID("field-obs", "batch-obs", "field", 0); err != nil {
		t.Fatal(err)
	}
	first, err := app.Obs.Import("batch-obs", "field-obs", model.UnitDegrees, []float64{10, 190}, "submission-1")
	if err != nil {
		t.Fatal(err)
	}
	if first.Inserted != 2 || first.Skipped != 0 {
		t.Fatalf("first import = %#v", first)
	}
	retry, err := app.Obs.Import("batch-obs", "field-obs", model.UnitDegrees, []float64{10, 190}, "submission-1")
	if err != nil {
		t.Fatal(err)
	}
	if retry.Inserted != 0 || retry.Skipped != 2 {
		t.Fatalf("retry import = %#v", retry)
	}
	if _, err := app.Obs.Import("batch-obs", "field-obs", model.UnitRadians, []float64{0.5}, "submission-2"); err != nil {
		t.Fatal(err)
	}
	count, err := app.Obs.CountByBatch("batch-obs")
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("observation count = %d, want 3", count)
	}
	units, err := app.Obs.DistinctUnits("batch-obs")
	if err != nil {
		t.Fatal(err)
	}
	if len(units) != 2 {
		t.Fatalf("distinct units = %#v, want deg and rad", units)
	}
}
