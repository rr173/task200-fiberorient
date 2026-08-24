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

// TestResultFreezeMetadataPersistsAcrossReload 验证冻结元数据的持久化链路：
// 冻结后通过 Get（重载自数据库）取回的结果，除状态保持 frozen 外，
// 还必须保留冻结时间——此前 FrozenAt 因 model/service/store 三处均
// 将其置 nil 而在重载后丢失。
func TestResultFreezeMetadataPersistsAcrossReload(t *testing.T) {
	app := newAcceptanceApp(t)
	if _, err := app.Batches.Create("batch-freeze-meta", "freeze", "paper", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Batches.Transition("batch-freeze-meta", model.BatchObserving); err != nil {
		t.Fatal(err)
	}
	for _, field := range []struct {
		id    string
		slice float64
		data  []float64
	}{{
		"field-freeze-0", 0, []float64{30, 31, 30, 29, 30},
	}, {
		"field-freeze-90", 90, []float64{120, 121, 120, 119, 120},
	}} {
		if _, err := app.Fields.RegisterWithID(field.id, "batch-freeze-meta", field.id, field.slice); err != nil {
			t.Fatal(err)
		}
		if _, err := app.Obs.Import("batch-freeze-meta", field.id, model.UnitDegrees, field.data, field.id+"-submission"); err != nil {
			t.Fatal(err)
		}
		if _, err := app.Fields.MarkValid(field.id); err != nil {
			t.Fatal(err)
		}
	}
	draft, err := app.Cals.EstimateAndDraft("batch-freeze-meta", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Cals.Activate(draft.Calibration.ID); err != nil {
		t.Fatal(err)
	}

	res, err := app.Results.Compute(result.ComputeOptions{
		BatchID:       "batch-freeze-meta",
		CalibrationID: draft.Calibration.ID,
		FieldIDs:      []string{"field-freeze-0", "field-freeze-90"},
		CiLevel:       0.95,
		Seed:          11,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != model.ResultPublishable {
		t.Fatalf("pre-freeze status = %s, want publishable", res.Status)
	}

	frozen, err := app.Results.Freeze(res.ID)
	if err != nil {
		t.Fatal(err)
	}
	if frozen.Status != model.ResultFrozen {
		t.Fatalf("freeze status = %s, want frozen", frozen.Status)
	}
	if frozen.FrozenAt == nil {
		t.Fatal("freeze response should carry FrozenAt, got nil")
	}
	stamped := *frozen.FrozenAt

	// 重载：经 Get 从数据库重新读取，状态与冻结时间都必须保留。
	// 调用 Latest 同样走 ListByBatch→scanResult，覆盖同一读取链路。
	reloaded, err := app.Results.Get(res.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Status != model.ResultFrozen {
		t.Fatalf("reload status = %s, want frozen", reloaded.Status)
	}
	if reloaded.FrozenAt == nil {
		t.Fatal("reload should preserve FrozenAt, got nil — freeze metadata lost")
	}
	if !reloaded.FrozenAt.Equal(stamped) {
		t.Fatalf("reload FrozenAt = %v, want %v (freeze-time must round-trip)",
			reloaded.FrozenAt, stamped)
	}
}
