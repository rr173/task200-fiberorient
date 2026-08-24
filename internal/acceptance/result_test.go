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

// TestResultExcludesExcludedFieldFromSnapshot 验证污染并剔除的视野不再
// 出现在结果的有效视野快照与观测数量中（清洗结果与统计输入一致性）。
func TestResultExcludesExcludedFieldFromSnapshot(t *testing.T) {
	app := newAcceptanceApp(t)
	if _, err := app.Batches.Create("batch-excl", "excluded", "paper", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Batches.Transition("batch-excl", model.BatchObserving); err != nil {
		t.Fatal(err)
	}
	for _, field := range []struct {
		id    string
		slice float64
		data  []float64
	}{
		{"field-excl-keep", 0, []float64{30, 31, 30, 29, 30}},
		{"field-excl-ninety", 90, []float64{120, 121, 120, 119, 120}},
		{"field-excl-drop", 90, []float64{122, 118, 121, 119, 120}},
	} {
		if _, err := app.Fields.RegisterWithID(field.id, "batch-excl", field.id, field.slice); err != nil {
			t.Fatal(err)
		}
		if _, err := app.Obs.Import("batch-excl", field.id, model.UnitDegrees, field.data, field.id+"-submission"); err != nil {
			t.Fatal(err)
		}
		if _, err := app.Fields.MarkValid(field.id); err != nil {
			t.Fatal(err)
		}
	}
	// 将第三个视野标记污染并剔除，校准仍可由前两个视野支撑。
	if _, err := app.Fields.MarkPolluted("field-excl-drop", "dust"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Fields.Exclude("field-excl-drop"); err != nil {
		t.Fatal(err)
	}

	draft, err := app.Cals.EstimateAndDraft("batch-excl", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Cals.Activate(draft.Calibration.ID); err != nil {
		t.Fatal(err)
	}

	// 编排层快照应仅含未剔除的两个视野，剔除的第三个不出现。
	fieldIDs, snap, err := app.FullStatSnapshot("batch-excl")
	if err != nil {
		t.Fatal(err)
	}
	wantIDs := []string{"field-excl-keep", "field-excl-ninety"}
	if len(fieldIDs) != 2 || fieldIDs[0] != wantIDs[0] || fieldIDs[1] != wantIDs[1] {
		t.Fatalf("stat field ids = %#v, want %#v", fieldIDs, wantIDs)
	}
	if snap != "field-excl-keep,field-excl-ninety" {
		t.Fatalf("snapshot = %q, want field-excl-keep,field-excl-ninety", snap)
	}

	res, err := app.Results.Compute(result.ComputeOptions{
		BatchID:       "batch-excl",
		CalibrationID: draft.Calibration.ID,
		FieldIDs:      fieldIDs,
		CiLevel:       0.95,
		Seed:          11,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.FieldSnapshot != "field-excl-keep,field-excl-ninety" {
		t.Fatalf("result field snapshot = %q, want field-excl-keep,field-excl-ninety", res.FieldSnapshot)
	}
	// 两个保留视野各 5 条观测（共 10）；剔除视野的 5 条必须被排除。
	if res.ObservationCount != 10 {
		t.Fatalf("observation count = %d, want 10", res.ObservationCount)
	}
	if res.Stats == nil || res.Stats.SampleSize != 10 {
		t.Fatalf("stats sample size = %v, want 10", res.Stats)
	}
}
