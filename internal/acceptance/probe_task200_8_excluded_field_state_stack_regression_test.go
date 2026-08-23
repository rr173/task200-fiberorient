package acceptance_test

import (
	"testing"

	"task200-fiberorient/internal/model"
)

func TestBug08_ExcludedFieldCannotBecomeValid(t *testing.T) {
	app := newAcceptanceApp(t)
	if _, err := app.Batches.Create("batch-bug8", "field state", "paper", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Fields.RegisterWithID("bug8-field", "batch-bug8", "field", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Fields.MarkPolluted("bug8-field", "dust"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Fields.Exclude("bug8-field"); err != nil {
		t.Fatal(err)
	}
	field, err := app.Fields.MarkValid("bug8-field")
	if err == nil {
		var invalid *model.Field
		_ = invalid.ID
	}
	if err == nil || field != nil {
		t.Fatal("excluded field unexpectedly became valid")
	}
	stored, err := app.Fields.Get("bug8-field")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != model.FieldExcluded {
		t.Fatalf("stored field status = %s, want excluded", stored.Status)
	}
}
