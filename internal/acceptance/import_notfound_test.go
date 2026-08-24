package acceptance_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"task200-fiberorient/internal/httpapi"
	"task200-fiberorient/internal/model"
)

func TestImportIntoNonexistentFieldReturnsNotFound(t *testing.T) {
	app := newAcceptanceApp(t)
	if _, err := app.Batches.Create("batch-x", "x", "paper", 0); err != nil {
		t.Fatal(err)
	}
	// 不创建任何视野，直接向不存在的 field 导入。
	// 服务层应返回资源不存在错误，而非 panic。
	_, err := app.Obs.Import("batch-x", "field-missing", model.UnitDegrees, []float64{10}, "sub-1")
	if !errors.Is(err, model.ErrNotFound) {
		t.Fatalf("service-level import err = %v, want ErrNotFound", err)
	}

	// HTTP 层应稳定返回 404，保持请求处理稳定（不 panic）。
	server := httptest.NewServer(httpapi.New(app).Handler())
	t.Cleanup(server.Close)
	body := strings.NewReader(`{"field_id":"field-missing","unit":"deg","angles":[10],"submission_id":"sub-1"}`)
	resp, err := http.Post(server.URL+"/api/batches/batch-x/observations/import", "application/json", body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("http status = %d, want 404 Not Found", resp.StatusCode)
	}
}
