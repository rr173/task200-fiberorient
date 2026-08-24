package httpapi

import (
	"net/http"
	"time"

	"task200-fiberorient/internal/model"
	"task200-fiberorient/internal/result"
)

// --- 批次 ---

type createBatchReq struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Material      string  `json:"material"`
	SliceAngleDeg float64 `json:"slice_angle_deg"`
}

func (a *API) createBatch(w http.ResponseWriter, r *http.Request) {
	var req createBatchReq
	if err := readJSON(r, &req); err != nil {
		writeErr(w, model.ErrBadInput)
		return
	}
	b, err := a.app.Batches.Create(req.ID, req.Name, req.Material, req.SliceAngleDeg)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, b)
}

func (a *API) listBatches(w http.ResponseWriter, r *http.Request) {
	batches, err := a.app.Batches.List()
	if err != nil {
		writeErr(w, err)
		return
	}
	if batches == nil {
		batches = []*model.Batch{}
	}
	writeJSON(w, http.StatusOK, batches)
}

func (a *API) getBatch(w http.ResponseWriter, r *http.Request) {
	b, err := a.app.Batches.Get(pathValue(r, "id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

type updateSliceReq struct {
	SliceAngleDeg float64 `json:"slice_angle_deg"`
}

func (a *API) updateSliceAngle(w http.ResponseWriter, r *http.Request) {
	var req updateSliceReq
	if err := readJSON(r, &req); err != nil {
		writeErr(w, model.ErrBadInput)
		return
	}
	b, err := a.app.Batches.UpdateSliceAngle(pathValue(r, "id"), req.SliceAngleDeg)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

type transitionReq struct {
	Target string `json:"target"`
}

func (a *API) transitionBatch(w http.ResponseWriter, r *http.Request) {
	var req transitionReq
	if err := readJSON(r, &req); err != nil {
		writeErr(w, model.ErrBadInput)
		return
	}
	b, err := a.app.Batches.Transition(pathValue(r, "id"), model.BatchStatus(req.Target))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

func (a *API) publishBatch(w http.ResponseWriter, r *http.Request) {
	b, err := a.app.Publish(pathValue(r, "id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

// --- 视野 ---

type createFieldReq struct {
	ID       string  `json:"id"`
	Label    string  `json:"label"`
	SliceDeg float64 `json:"slice_deg"`
}

func (a *API) createField(w http.ResponseWriter, r *http.Request) {
	batchID := pathValue(r, "id")
	var req createFieldReq
	if err := readJSON(r, &req); err != nil {
		writeErr(w, model.ErrBadInput)
		return
	}
	var (
		f   *model.Field
		err error
	)
	if req.ID != "" {
		f, err = a.app.Fields.RegisterWithID(req.ID, batchID, req.Label, req.SliceDeg)
	} else {
		f, err = a.app.Fields.Register(batchID, req.Label, req.SliceDeg)
	}
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, f)
}

func (a *API) listFields(w http.ResponseWriter, r *http.Request) {
	fields, err := a.app.Fields.ListByBatch(pathValue(r, "id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	if fields == nil {
		fields = []*model.Field{}
	}
	writeJSON(w, http.StatusOK, fields)
}

func (a *API) getField(w http.ResponseWriter, r *http.Request) {
	f, err := a.app.Fields.Get(pathValue(r, "id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, f)
}

func (a *API) markFieldValid(w http.ResponseWriter, r *http.Request) {
	f, err := a.app.Fields.MarkValid(pathValue(r, "id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, f)
}

type markPollutedReq struct {
	Reason string `json:"reason"`
}

func (a *API) markFieldPolluted(w http.ResponseWriter, r *http.Request) {
	var req markPollutedReq
	if err := readJSON(r, &req); err != nil {
		writeErr(w, model.ErrBadInput)
		return
	}
	f, err := a.app.Fields.MarkPolluted(pathValue(r, "id"), req.Reason)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, f)
}

func (a *API) excludeField(w http.ResponseWriter, r *http.Request) {
	f, err := a.app.Fields.Exclude(pathValue(r, "id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, f)
}

// --- 观测 ---

type importReq struct {
	FieldID      string    `json:"field_id"`
	Unit         string    `json:"unit"`
	Angles       []float64 `json:"angles"`
	SubmissionID string    `json:"submission_id"`
}

func (a *API) importObservations(w http.ResponseWriter, r *http.Request) {
	batchID := pathValue(r, "id")
	var req importReq
	if err := readJSON(r, &req); err != nil {
		writeErr(w, model.ErrBadInput)
		return
	}
	res, err := a.app.Obs.Import(batchID, req.FieldID, model.AngleUnit(req.Unit), req.Angles, req.SubmissionID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (a *API) listObservations(w http.ResponseWriter, r *http.Request) {
	obs, err := a.app.Obs.ListByBatch(pathValue(r, "id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	if obs == nil {
		obs = []*model.Observation{}
	}
	writeJSON(w, http.StatusOK, obs)
}

func (a *API) countObservations(w http.ResponseWriter, r *http.Request) {
	n, err := a.app.Obs.CountByBatch(pathValue(r, "id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"count": n})
}

func (a *API) distinctUnits(w http.ResponseWriter, r *http.Request) {
	units, err := a.app.Obs.DistinctUnits(pathValue(r, "id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	if units == nil {
		units = []model.AngleUnit{}
	}
	writeJSON(w, http.StatusOK, units)
}

// --- 校准 ---

func (a *API) createCalibration(w http.ResponseWriter, r *http.Request) {
	batchID := pathValue(r, "id")
	res, err := a.app.Cals.EstimateAndDraft(batchID, 0)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

func (a *API) listCalibrations(w http.ResponseWriter, r *http.Request) {
	cals, err := a.app.Cals.ListByBatch(pathValue(r, "id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	if cals == nil {
		cals = []*model.Calibration{}
	}
	writeJSON(w, http.StatusOK, cals)
}

func (a *API) getCalibration(w http.ResponseWriter, r *http.Request) {
	c, err := a.app.Cals.Get(pathValue(r, "id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (a *API) activateCalibration(w http.ResponseWriter, r *http.Request) {
	c, err := a.app.Cals.Activate(pathValue(r, "id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (a *API) revokeCalibration(w http.ResponseWriter, r *http.Request) {
	c, err := a.app.Cals.Revoke(pathValue(r, "id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// --- 统计与结果 ---

type computeReq struct {
	CiLevel float64 `json:"ci_level"`
	Seed    int64   `json:"seed"`
}

func (a *API) computeResult(w http.ResponseWriter, r *http.Request) {
	batchID := pathValue(r, "id")
	var req computeReq
	req.CiLevel = 0.95
	_ = readJSON(r, &req) // 空体允许：使用默认参数
	cal, err := a.app.Cals.Active(batchID)
	if err != nil {
		writeErr(w, err)
		return
	}
	fieldIDs, _, err := a.app.FullStatSnapshot(batchID)
	if err != nil {
		writeErr(w, err)
		return
	}
	res, err := a.app.Results.Compute(result.ComputeOptions{
		BatchID:       batchID,
		CalibrationID: cal.ID,
		FieldIDs:      fieldIDs,
		CiLevel:       req.CiLevel,
		Seed:          req.Seed,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

func (a *API) listResults(w http.ResponseWriter, r *http.Request) {
	results, err := a.app.Results.ListByBatch(pathValue(r, "id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	if results == nil {
		results = []*model.Result{}
	}
	writeJSON(w, http.StatusOK, results)
}

func (a *API) latestResult(w http.ResponseWriter, r *http.Request) {
	res, err := a.app.Results.Latest(pathValue(r, "id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (a *API) getResult(w http.ResponseWriter, r *http.Request) {
	res, err := a.app.Results.Get(pathValue(r, "id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (a *API) freezeResult(w http.ResponseWriter, r *http.Request) {
	res, err := a.app.Results.Freeze(pathValue(r, "id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (a *API) compareResults(w http.ResponseWriter, r *http.Request) {
	left := r.URL.Query().Get("left")
	right := r.URL.Query().Get("right")
	if left == "" || right == "" {
		writeErr(w, model.ErrBadInput)
		return
	}
	cmp, err := a.app.Results.Compare(left, right)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cmp)
}

// --- 自检 ---

func (a *API) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) selfcheck(w http.ResponseWriter, r *http.Request) {
	db := a.app.DB()
	var n int
	if err := db.SQL().QueryRow("SELECT COUNT(*) FROM batches").Scan(&n); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":        "ok",
		"batch_count":   n,
		"db_path":       db.Path(),
		"checked_at_ts": time.Now().UTC().Unix(),
	})
}
