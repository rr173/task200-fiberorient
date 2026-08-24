package acceptance_test

import (
	"testing"

	"task200-fiberorient/internal/model"
)

// TestBatchSliceAngleUpdateAdvancesVersion 验证批次切片方向更新后版本号向前推进，
// 且随后用更新前读到的旧批次对象写入会被乐观锁拒绝（ErrConflict）。
// 这条不变式此前被破坏：版本未前移，旧对象仍可覆盖新状态。
func TestBatchSliceAngleUpdateAdvancesVersion(t *testing.T) {
	app := newAcceptanceApp(t)
	if _, err := app.Batches.Create("batch-occ", "occ", "paper", 0); err != nil {
		t.Fatal(err)
	}

	before, err := app.Batches.Get("batch-occ")
	if err != nil {
		t.Fatal(err)
	}
	stale := *before // 更新前读到的旧批次对象（保留旧 version）

	updated, err := app.Batches.UpdateSliceAngle("batch-occ", 45)
	if err != nil {
		t.Fatalf("update slice angle: %v", err)
	}
	if updated.Version != before.Version+1 {
		t.Fatalf("version should advance by 1 on slice angle update: before=%d after=%d",
			before.Version, updated.Version)
	}
	if updated.SliceAngleDeg != 45 {
		t.Fatalf("slice angle not applied: got %v", updated.SliceAngleDeg)
	}
	if updated.Status != model.BatchObserving {
		t.Fatalf("status should be observing after slice angle update, got %s", updated.Status)
	}

	// 持久层版本也必须前移：重新读取得到新版本。
	reread, err := app.Batches.Get("batch-occ")
	if err != nil {
		t.Fatal(err)
	}
	if reread.Version != updated.Version {
		t.Fatalf("persisted version %d != returned %d", reread.Version, updated.Version)
	}

	// 用更新前读到的旧对象（携带旧 version）再次写入，必须被乐观锁拒绝。
	// 复用底层 store：直接调用 BatchStore.Update 以模拟“旧对象写入”路径。
	stale.Name = "overwritten-by-stale"
	err = app.Batches.Update(&stale)
	if err != model.ErrConflict {
		t.Fatalf("stale-object write must be rejected with ErrConflict, got %v", err)
	}

	// 旧对象写被拒后，新状态不应被覆盖。
	final, err := app.Batches.Get("batch-occ")
	if err != nil {
		t.Fatal(err)
	}
	if final.Name != before.Name {
		t.Fatalf("stale object overwrote new state: name=%q", final.Name)
	}
	if final.SliceAngleDeg != 45 {
		t.Fatalf("stale object overwrote slice angle: got %v", final.SliceAngleDeg)
	}
	if final.Version != updated.Version {
		t.Fatalf("version regressed after rejected stale write: got %d want %d",
			final.Version, updated.Version)
	}
}

// TestBatchTransitionAdvancesVersion 验证状态推进同样前移版本号，且旧对象写入被拒。
func TestBatchTransitionAdvancesVersion(t *testing.T) {
	app := newAcceptanceApp(t)
	if _, err := app.Batches.Create("batch-tx", "tx", "paper", 0); err != nil {
		t.Fatal(err)
	}

	before, err := app.Batches.Get("batch-tx")
	if err != nil {
		t.Fatal(err)
	}
	stale := *before

	updated, err := app.Batches.Transition("batch-tx", model.BatchObserving)
	if err != nil {
		t.Fatalf("transition: %v", err)
	}
	if updated.Version != before.Version+1 {
		t.Fatalf("version should advance by 1 on transition: before=%d after=%d",
			before.Version, updated.Version)
	}

	stale.Material = "overwritten-by-stale"
	if err := app.Batches.Update(&stale); err != model.ErrConflict {
		t.Fatalf("stale-object write must be rejected with ErrConflict, got %v", err)
	}

	final, err := app.Batches.Get("batch-tx")
	if err != nil {
		t.Fatal(err)
	}
	if final.Material != before.Material {
		t.Fatalf("stale object overwrote new state: material=%q", final.Material)
	}
	if final.Version != updated.Version {
		t.Fatalf("version regressed after rejected stale write: got %d want %d",
			final.Version, updated.Version)
	}
}
