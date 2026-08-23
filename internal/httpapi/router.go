// Package httpapi HTTP 暴露层：路由前缀 /api，统一 JSON 出入与错误映射。
package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"task200-fiberorient/internal/model"
	"task200-fiberorient/internal/service"
)

// API HTTP 处理器集合。
type API struct {
	app *service.App
	mux *http.ServeMux
}

// New 构造 HTTP API。
func New(app *service.App) *API {
	a := &API{app: app, mux: http.NewServeMux()}
	a.routes()
	return a
}

// Handler 返回 http.Handler（含日志中间件）。
func (a *API) Handler() http.Handler {
	return logMiddleware(a.mux)
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

// routes 注册全部路由（≥20 个 API）。
func (a *API) routes() {
	// 批次
	a.mux.HandleFunc("POST /api/batches", a.createBatch)
	a.mux.HandleFunc("GET /api/batches", a.listBatches)
	a.mux.HandleFunc("GET /api/batches/{id}", a.getBatch)
	a.mux.HandleFunc("PATCH /api/batches/{id}/slice", a.updateSliceAngle)
	a.mux.HandleFunc("POST /api/batches/{id}/transition", a.transitionBatch)
	a.mux.HandleFunc("POST /api/batches/{id}/publish", a.publishBatch)

	// 视野
	a.mux.HandleFunc("POST /api/batches/{id}/fields", a.createField)
	a.mux.HandleFunc("GET /api/batches/{id}/fields", a.listFields)
	a.mux.HandleFunc("GET /api/fields/{id}", a.getField)
	a.mux.HandleFunc("POST /api/fields/{id}/valid", a.markFieldValid)
	a.mux.HandleFunc("POST /api/fields/{id}/polluted", a.markFieldPolluted)
	a.mux.HandleFunc("POST /api/fields/{id}/exclude", a.excludeField)

	// 观测导入与查询
	a.mux.HandleFunc("POST /api/batches/{id}/observations/import", a.importObservations)
	a.mux.HandleFunc("GET /api/batches/{id}/observations", a.listObservations)
	a.mux.HandleFunc("GET /api/batches/{id}/observations/count", a.countObservations)
	a.mux.HandleFunc("GET /api/batches/{id}/observations/units", a.distinctUnits)

	// 校准
	a.mux.HandleFunc("POST /api/batches/{id}/calibrations", a.createCalibration)
	a.mux.HandleFunc("GET /api/batches/{id}/calibrations", a.listCalibrations)
	a.mux.HandleFunc("GET /api/calibrations/{id}", a.getCalibration)
	a.mux.HandleFunc("POST /api/calibrations/{id}/activate", a.activateCalibration)
	a.mux.HandleFunc("POST /api/calibrations/{id}/revoke", a.revokeCalibration)

	// 统计与结果
	a.mux.HandleFunc("POST /api/batches/{id}/compute", a.computeResult)
	a.mux.HandleFunc("GET /api/batches/{id}/results", a.listResults)
	a.mux.HandleFunc("GET /api/batches/{id}/results/latest", a.latestResult)
	a.mux.HandleFunc("GET /api/results/{id}", a.getResult)
	a.mux.HandleFunc("POST /api/results/{id}/freeze", a.freezeResult)
	a.mux.HandleFunc("GET /api/results/compare", a.compareResults)

	// 自检与健康
	a.mux.HandleFunc("GET /api/health", a.health)
	a.mux.HandleFunc("POST /api/selfcheck", a.selfcheck)
}

// writeJSON 输出 JSON。
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write json: %v", err)
	}
}

// writeErr 统一错误映射。
func writeErr(w http.ResponseWriter, err error) {
	var code int
	switch {
	case errors.Is(err, model.ErrNotFound):
		code = http.StatusNotFound
	case errors.Is(err, model.ErrConflict):
		code = http.StatusConflict
	case errors.Is(err, model.ErrInvalidState):
		code = http.StatusConflict
	case errors.Is(err, model.ErrInsufficientData):
		code = http.StatusUnprocessableEntity
	case errors.Is(err, model.ErrInvalidArgument), errors.Is(err, model.ErrBadInput):
		code = http.StatusBadRequest
	default:
		code = http.StatusInternalServerError
	}
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

// readJSON 解析请求体。
func readJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(v); err != nil {
		return err
	}
	return nil
}

func pathValue(r *http.Request, key string) string {
	return r.PathValue(key)
}
