package acceptance_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"task200-fiberorient/internal/httpapi"
)

func TestHTTPServerMountsHealthAndBatchWriteRoutes(t *testing.T) {
	app := newAcceptanceApp(t)
	server := httptest.NewServer(httpapi.New(app).Handler())
	t.Cleanup(server.Close)

	resp, err := http.Get(server.URL + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("health status = %d", resp.StatusCode)
	}
	resp.Body.Close()

	body := strings.NewReader(`{"id":"batch-http","name":"HTTP","material":"paper","slice_angle_deg":45}`)
	resp, err = http.Post(server.URL+"/api/batches", "application/json", body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create batch status = %d", resp.StatusCode)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.ID != "batch-http" {
		t.Fatalf("created batch id = %q", created.ID)
	}
}
