package acceptance_test

import (
	"errors"
	"sync"
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

// TestResultComputeConcurrentVersioning 验证同一批次同时启动 20 个结果计算请求时，
// 每个请求获得唯一且连续的版本号（1..20），无重复版本、无唯一键冲突错误。
// 回归 bug7：旧实现 NextVersion+Insert 两步分配在并发下产生重复版本与
// UNIQUE(batch_id, version) 冲突。
func TestResultComputeConcurrentVersioning(t *testing.T) {
	app := newAcceptanceApp(t)
	if _, err := app.Batches.Create("batch-conc", "concurrent", "paper", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Batches.Transition("batch-conc", model.BatchObserving); err != nil {
		t.Fatal(err)
	}
	for _, field := range []struct {
		id    string
		slice float64
		data  []float64
	}{{"field-conc-0", 0, []float64{30, 31, 30, 29, 30}}, {"field-conc-90", 90, []float64{120, 121, 120, 119, 120}}} {
		if _, err := app.Fields.RegisterWithID(field.id, "batch-conc", field.id, field.slice); err != nil {
			t.Fatal(err)
		}
		if _, err := app.Obs.Import("batch-conc", field.id, model.UnitDegrees, field.data, field.id+"-submission"); err != nil {
			t.Fatal(err)
		}
		if _, err := app.Fields.MarkValid(field.id); err != nil {
			t.Fatal(err)
		}
	}
	draft, err := app.Cals.EstimateAndDraft("batch-conc", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Cals.Activate(draft.Calibration.ID); err != nil {
		t.Fatal(err)
	}

	const n = 20
	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		versions  = map[int]int{}
		conflicts int
		otherErrs []error
	)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			res, err := app.Results.Compute(result.ComputeOptions{
				BatchID:       "batch-conc",
				CalibrationID: draft.Calibration.ID,
				FieldIDs:      []string{"field-conc-0", "field-conc-90"},
				CiLevel:       0.95,
				Seed:          int64(i),
			})
			if err != nil {
				mu.Lock()
				if errors.Is(err, model.ErrConflict) {
					conflicts++
				} else {
					otherErrs = append(otherErrs, err)
				}
				mu.Unlock()
				return
			}
			mu.Lock()
			versions[res.Version]++
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	if len(otherErrs) != 0 {
		t.Fatalf("unexpected errors: %v", otherErrs)
	}
	if conflicts != 0 {
		t.Fatalf("got %d unique-key conflicts, want 0", conflicts)
	}
	if len(versions) != n {
		t.Fatalf("got %d distinct versions, want %d (duplicates present)", len(versions), n)
	}
	for v := 1; v <= n; v++ {
		if versions[v] != 1 {
			t.Fatalf("version %d allocated %d times, want exactly 1", v, versions[v])
		}
	}
}
