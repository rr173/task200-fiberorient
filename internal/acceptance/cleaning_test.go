package acceptance_test

import (
	"testing"

	"task200-fiberorient/internal/model"
)

func TestCleaningExcludesPollutedFieldsFromSnapshot(t *testing.T) {
	app := newAcceptanceApp(t)
	if _, err := app.Batches.Create("batch-clean", "cleaning", "paper", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Fields.RegisterWithID("field-good", "batch-clean", "good", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Fields.RegisterWithID("field-bad", "batch-clean", "bad", 90); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Fields.MarkValid("field-good"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Fields.MarkPolluted("field-bad", "dust"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Fields.Exclude("field-bad"); err != nil {
		t.Fatal(err)
	}

	fields, err := app.Fields.EffectiveFields("batch-clean")
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) != 1 || fields[0].ID != "field-good" || fields[0].Status != model.FieldValid {
		t.Fatalf("effective fields = %#v", fields)
	}
	snapshot, err := app.Fields.Snapshot("batch-clean")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot != "field-good" {
		t.Fatalf("snapshot = %q, want field-good", snapshot)
	}
}
