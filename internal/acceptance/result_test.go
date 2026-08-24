package acceptance_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"task200-fiberorient/internal/httpapi"
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

// TestLatestResultReturnsNewestVersion 锁定修复：批次存在多个结果版本时，
// Latest 必须返回版本号最大（最新）的那一个，而非更早版本；HTTP
// /results/latest 响应中的 version 不得被减一调整。
func TestLatestResultReturnsNewestVersion(t *testing.T) {
	app := newAcceptanceApp(t)
	if _, err := app.Batches.Create("batch-latest", "latest", "paper", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Batches.Transition("batch-latest", model.BatchObserving); err != nil {
		t.Fatal(err)
	}
	for _, field := range []struct {
		id    string
		slice float64
		data  []float64
	}{{"fld-latest-0", 0, []float64{30, 31, 30, 29, 30}}, {"fld-latest-90", 90, []float64{120, 121, 120, 119, 120}}} {
		if _, err := app.Fields.RegisterWithID(field.id, "batch-latest", field.id, field.slice); err != nil {
			t.Fatal(err)
		}
		if _, err := app.Obs.Import("batch-latest", field.id, model.UnitDegrees, field.data, field.id+"-submission"); err != nil {
			t.Fatal(err)
		}
		if _, err := app.Fields.MarkValid(field.id); err != nil {
			t.Fatal(err)
		}
	}
	draft, err := app.Cals.EstimateAndDraft("batch-latest", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Cals.Activate(draft.Calibration.ID); err != nil {
		t.Fatal(err)
	}
	opts := result.ComputeOptions{
		BatchID:       "batch-latest",
		CalibrationID: draft.Calibration.ID,
		FieldIDs:      []string{"fld-latest-0", "fld-latest-90"},
		CiLevel:      0.95,
		Seed:         11,
	}
	v1, err := app.Results.Compute(opts)
	if err != nil {
		t.Fatal(err)
	}
	if v1.Version != 1 {
		t.Fatalf("first result version = %d, want 1", v1.Version)
	}
	opts.Seed = 12
	v2, err := app.Results.Compute(opts)
	if err != nil {
		t.Fatal(err)
	}
	if v2.Version != 2 {
		t.Fatalf("second result version = %d, want 2", v2.Version)
	}
	opts.Seed = 13
	v3, err := app.Results.Compute(opts)
	if err != nil {
		t.Fatal(err)
	}
	if v3.Version != 3 {
		t.Fatalf("third result version = %d, want 3", v3.Version)
	}

	// Latest 必须返回 v3（最新版本），而非 v1 或 v2。
	latest, err := app.Results.Latest("batch-latest")
	if err != nil {
		t.Fatal(err)
	}
	if latest.Version != 3 {
		t.Fatalf("Latest version = %d, want 3", latest.Version)
	}
	if latest.ID != v3.ID {
		t.Fatalf("Latest id = %q, want %q", latest.ID, v3.ID)
	}

	// 列表返回的第一项也应为最新版本（DESC 排序约定）。
	list, err := app.Results.ListByBatch("batch-latest")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 || list[0].Version != 3 {
		t.Fatalf("ListByBatch order wrong: len=%d first=%d", len(list), firstOrZero(list))
	}

	// HTTP /results/latest 不得把版本号减一。
	server := httptest.NewServer(httpapi.New(app).Handler())
	t.Cleanup(server.Close)
	resp, err := http.Get(server.URL + "/api/batches/batch-latest/results/latest")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("latest http status = %d", resp.StatusCode)
	}
	var got struct {
		ID      string `json:"id"`
		Version int    `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Version != 3 {
		t.Fatalf("http latest version = %d, want 3 (must not be decremented)", got.Version)
	}
	if got.ID != v3.ID {
		t.Fatalf("http latest id = %q, want %q", got.ID, v3.ID)
	}
}

func firstOrZero(rs []*model.Result) int {
	if len(rs) == 0 {
		return 0
	}
	return rs[0].Version
}
