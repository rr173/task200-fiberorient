package acceptance_test

import (
	"testing"

	"task200-fiberorient/internal/model"
	"task200-fiberorient/internal/service"
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

// setupCalBatch 建好可校准批次，返回新草稿所需调用。
func setupCalBatch(t *testing.T, app *service.App, batchID string) {
	t.Helper()
	if _, err := app.Batches.Create(batchID, "calibration", "paper", 45); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Batches.Transition(batchID, model.BatchObserving); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Fields.RegisterWithID(batchID+"-f0", batchID, "zero", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Fields.RegisterWithID(batchID+"-f90", batchID, "ninety", 90); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Obs.Import(batchID, batchID+"-f0", model.UnitDegrees, []float64{40, 41, 42}, "s0"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Obs.Import(batchID, batchID+"-f90", model.UnitDegrees, []float64{108, 109, 110}, "s90"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Fields.MarkValid(batchID + "-f0"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Fields.MarkValid(batchID + "-f90"); err != nil {
		t.Fatal(err)
	}
}

// TestCalibrationActivateRevokesPrevious 同一批次连续激活两版后，旧生效版本必须自动废止，
// 任意时刻只有一个生效校准。回归：旧实现 RevokeAllActive 筛选条件错误，连续激活后留下多个 active。
func TestCalibrationActivateRevokesPrevious(t *testing.T) {
	app := newAcceptanceApp(t)
	const batchID = "batch-multi"
	setupCalBatch(t, app, batchID)

	// 第一版
	d1, err := app.Cals.EstimateAndDraft(batchID, 0)
	if err != nil {
		t.Fatal(err)
	}
	a1, err := app.Cals.Activate(d1.Calibration.ID)
	if err != nil {
		t.Fatal(err)
	}
	if a1.Status != model.CalibrationActive || a1.ActivatedAt == nil {
		t.Fatalf("first activation: status=%s activated_at=%v", a1.Status, a1.ActivatedAt)
	}

	// 第二版
	d2, err := app.Cals.EstimateAndDraft(batchID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if d2.Calibration.ID == d1.Calibration.ID {
		t.Fatalf("second draft reused same id %s", d2.Calibration.ID)
	}
	a2, err := app.Cals.Activate(d2.Calibration.ID)
	if err != nil {
		t.Fatal(err)
	}
	if a2.Status != model.CalibrationActive {
		t.Fatalf("second activation status = %s", a2.Status)
	}

	// 当前生效的只能是第二版
	cur, err := app.Cals.Active(batchID)
	if err != nil {
		t.Fatal(err)
	}
	if cur.ID != a2.ID {
		t.Fatalf("active calibration = %s, want %s", cur.ID, a2.ID)
	}

	// 列举整批，统计 active 数量必须为 1，旧版本必须已废止
	list, err := app.Cals.ListByBatch(batchID)
	if err != nil {
		t.Fatal(err)
	}
	var activeCount int
	var oldRevoked bool
	for _, c := range list {
		if c.Status == model.CalibrationActive {
			activeCount++
		}
		if c.ID == d1.Calibration.ID {
			if c.Status == model.CalibrationRevoked {
				oldRevoked = true
			}
			if c.RevokedAt == nil {
				t.Fatalf("old calibration %s not stamped revoked_at", c.ID)
			}
		}
	}
	if activeCount != 1 {
		t.Fatalf("active count = %d, want exactly 1", activeCount)
	}
	if !oldRevoked {
		t.Fatalf("previous calibration %s not revoked", d1.Calibration.ID)
	}
}
