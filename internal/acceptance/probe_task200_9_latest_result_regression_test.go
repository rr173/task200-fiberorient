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

func TestBug09_LatestResultReturnsNewestVersion(t *testing.T) {
	app := newAcceptanceApp(t)
	if _, err := app.Batches.Create("batch-bug9", "latest", "paper", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Batches.Transition("batch-bug9", model.BatchObserving); err != nil {
		t.Fatal(err)
	}
	for _, field := range []struct {
		id    string
		slice float64
		angle float64
	}{{"bug9-field-0", 0, 30}, {"bug9-field-90", 90, 120}} {
		if _, err := app.Fields.RegisterWithID(field.id, "batch-bug9", field.id, field.slice); err != nil {
			t.Fatal(err)
		}
		if _, err := app.Obs.Import("batch-bug9", field.id, model.UnitDegrees, []float64{field.angle, field.angle, field.angle, field.angle, field.angle}, field.id+"-submission"); err != nil {
			t.Fatal(err)
		}
		if _, err := app.Fields.MarkValid(field.id); err != nil {
			t.Fatal(err)
		}
	}
	draft, err := app.Cals.EstimateAndDraft("batch-bug9", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Cals.Activate(draft.Calibration.ID); err != nil {
		t.Fatal(err)
	}
	for seed := int64(1); seed <= 2; seed++ {
		if _, err := app.Results.Compute(result.ComputeOptions{BatchID: "batch-bug9", CalibrationID: draft.Calibration.ID, FieldIDs: []string{"bug9-field-0", "bug9-field-90"}, CiLevel: 0.95, Seed: seed}); err != nil {
			t.Fatal(err)
		}
	}
	server := httptest.NewServer(httpapi.New(app).Handler())
	t.Cleanup(server.Close)
	resp, err := http.Get(server.URL + "/api/batches/batch-bug9/results/latest")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var got struct {
		Version int `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Version != 2 {
		t.Fatalf("latest version = %d, want 2", got.Version)
	}
}
