package acceptance_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"task200-fiberorient/internal/httpapi"
)

func TestBug02_MissingFieldReturnsNotFoundWithoutPanic(t *testing.T) {
	app := newAcceptanceApp(t)
	if _, err := app.Batches.Create("batch-bug2", "missing field", "paper", 0); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(httpapi.New(app).Handler())
	t.Cleanup(server.Close)
	body := strings.NewReader(`{"field_id":"does-not-exist","unit":"deg","angles":[12],"submission_id":"missing-field"}`)
	resp, err := http.Post(server.URL+"/api/batches/batch-bug2/observations/import", "application/json", body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}
