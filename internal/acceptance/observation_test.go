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

// TestObservationImportCrossFieldSameSubmission 验证跨视野导入场景：
// 同一批次中两个不同切片视野使用相同的提交标识（submission_id）导入观测，
// 两次导入都属于不同视野，应各自成功保留；仅同一视野内同提交同序号重试才幂等跳过。
func TestObservationImportCrossFieldSameSubmission(t *testing.T) {
	app := newAcceptanceApp(t)
	if _, err := app.Batches.Create("batch-xfield", "cross-field", "paper", 45); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Fields.RegisterWithID("field-0", "batch-xfield", "0° slice", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Fields.RegisterWithID("field-90", "batch-xfield", "90° slice", 90); err != nil {
		t.Fatal(err)
	}
	angles := []float64{42, 43, 41}
	// 两个不同视野使用相同提交标识导入相同角度序列。
	first, err := app.Obs.Import("batch-xfield", "field-0", model.UnitDegrees, angles, "shared-sub")
	if err != nil {
		t.Fatal(err)
	}
	if first.Inserted != 3 || first.Skipped != 0 {
		t.Fatalf("first field import = %#v, want inserted=3 skipped=0", first)
	}
	second, err := app.Obs.Import("batch-xfield", "field-90", model.UnitDegrees, angles, "shared-sub")
	if err != nil {
		t.Fatal(err)
	}
	if second.Inserted != 3 || second.Skipped != 0 {
		t.Fatalf("second field import = %#v, want inserted=3 skipped=0 (cross-field, not duplicate)", second)
	}

	count, err := app.Obs.CountByBatch("batch-xfield")
	if err != nil {
		t.Fatal(err)
	}
	if count != 6 {
		t.Fatalf("observation count = %d, want 6 (3 per field)", count)
	}

	// 跨视野观测各自落在对应视野下。
	by0, err := app.Obs.ListByBatch("batch-xfield")
	if err != nil {
		t.Fatal(err)
	}
	var per0, per90 int
	for _, o := range by0 {
		switch o.FieldID {
		case "field-0":
			per0++
		case "field-90":
			per90++
		}
	}
	if per0 != 3 || per90 != 3 {
		t.Fatalf("per-field counts = field-0:%d field-90:%d, want 3/3", per0, per90)
	}

	// 同一视野内同提交同序号重试仍应幂等跳过（跨视野放宽不影响幂等不变量）。
	retry, err := app.Obs.Import("batch-xfield", "field-0", model.UnitDegrees, angles, "shared-sub")
	if err != nil {
		t.Fatal(err)
	}
	if retry.Inserted != 0 || retry.Skipped != 3 {
		t.Fatalf("same-field retry = %#v, want inserted=0 skipped=3 (idempotent)", retry)
	}
	if n, err := app.Obs.CountByBatch("batch-xfield"); err != nil || n != 6 {
		t.Fatalf("after same-field retry count = %d, want 6 unchanged", n)
	}
}
