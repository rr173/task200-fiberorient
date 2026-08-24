// Package result 结果模块：执行统计计算、生成结果版本（computing →
// insufficient_confidence / publishable → frozen），支持版本比较与发布。
package result

import (
	"errors"
	"fmt"

	"task200-fiberorient/internal/model"
	"task200-fiberorient/internal/statistics"
	"task200-fiberorient/internal/store"
)

// Service 结果模块服务。
type Service struct {
	results *store.ResultStore
	cals    *store.CalibrationStore
	obs     *store.ObservationStore
	fields  *store.FieldStore
}

// New 构造结果服务。
func New(results *store.ResultStore, cals *store.CalibrationStore, obs *store.ObservationStore, fields *store.FieldStore) *Service {
	return &Service{results: results, cals: cals, obs: obs, fields: fields}
}

// MinConfWidth 统计结果可发布的最大置信区间宽度（度）。
const MinConfWidth = 45.0

// ComputeOptions 计算参数。
type ComputeOptions struct {
	BatchID       string
	CalibrationID string
	FieldIDs      []string // 参与统计的视野（已筛选）
	CiLevel       float64
	Seed          int64
}

// Compute 执行一次统计并生成新结果版本（版本号串行递增）。
func (s *Service) Compute(opts ComputeOptions) (*model.Result, error) {
	if opts.BatchID == "" {
		return nil, fmt.Errorf("%w: batch id required", model.ErrInvalidArgument)
	}
	if opts.CalibrationID == "" {
		return nil, fmt.Errorf("%w: calibration id required", model.ErrInvalidArgument)
	}
	if len(opts.FieldIDs) == 0 {
		return nil, fmt.Errorf("%w: no fields selected for statistics", model.ErrInsufficientData)
	}
	cal, err := s.cals.Get(opts.CalibrationID)
	if err != nil {
		return nil, err
	}
	if cal.BatchID != opts.BatchID {
		return nil, fmt.Errorf("%w: calibration belongs to another batch", model.ErrInvalidArgument)
	}
	if cal.Status != model.CalibrationActive {
		return nil, fmt.Errorf("%w: only active calibration can drive statistics", model.ErrInvalidState)
	}

	obs, err := s.obs.ListByBatchFields(opts.BatchID, opts.FieldIDs)
	if err != nil {
		return nil, err
	}
	if len(obs) == 0 {
		return nil, fmt.Errorf("%w: no observations in selected fields", model.ErrInsufficientData)
	}

	// 单位混用拒绝：批次内观测单位必须统一。
	units := map[model.AngleUnit]bool{}
	for _, o := range obs {
		units[o.Unit] = true
	}
	if len(units) > 1 {
		return nil, fmt.Errorf("%w: mixed angle units in observations", model.ErrInvalidArgument)
	}

	// 按切片方向校正为面内取向：0° 切片 θ−δ；90° 切片 (θ−90)+δ。
	// 不同切片方向的表观角相差 90° 相位，必须统一到基准坐标系才能合并统计。
	sliceDeg := map[string]float64{}
	for _, fid := range opts.FieldIDs {
		f, err := s.fields.Get(fid)
		if err != nil {
			return nil, err
		}
		sliceDeg[fid] = f.SliceDeg
	}
	angles := make([]float64, 0, len(obs))
	for _, o := range obs {
		angles = append(angles, statistics.CorrectForSlice(o.AngleDeg, sliceDeg[o.FieldID], cal.BiasDeg))
	}

	// 版本号由 AllocateAndInsert 在 BEGIN IMMEDIATE 事务内原子分配：取
	// MAX(version)+1 与插入同一写锁下串行完成，杜绝并发重复版本与
	// UNIQUE(batch_id, version) 冲突。结果 ID（res-<batch>-v<version>）依赖版本号，
	// 亦在事务内确定版本后回写。此处先以占位版本 1 构造记录，真实版本号与 ID
	// 均由 AllocateAndInsert 在落库时写入。
	snapshot := joinIDs(opts.FieldIDs)
	res, err := model.NewResult("", opts.BatchID, 1, opts.CalibrationID, snapshot, len(obs))
	if err != nil {
		return nil, err
	}

	stats, bim, ci := statistics.Compute(angles, opts.CiLevel, opts.Seed)
	if err := res.Finalize(stats, bim, ci, MinConfWidth); err != nil {
		return nil, err
	}
	if err := s.results.AllocateAndInsert(res); err != nil {
		return nil, err
	}
	return res, nil
}

// Get 取结果详情。
func (s *Service) Get(id string) (*model.Result, error) {
	return s.results.Get(id)
}

// ListByBatch 列出批次结果版本。
func (s *Service) ListByBatch(batchID string) ([]*model.Result, error) {
	return s.results.ListByBatch(batchID)
}

// Freeze 冻结结果（终态）。
func (s *Service) Freeze(id string) (*model.Result, error) {
	r, err := s.results.Get(id)
	if err != nil {
		return nil, err
	}
	if err := r.Freeze(); err != nil {
		return nil, wrapInvalid(err)
	}
	if err := s.results.Update(r); err != nil {
		return nil, err
	}
	return r, nil
}

// Latest 返回批次最新结果版本。
func (s *Service) Latest(batchID string) (*model.Result, error) {
	all, err := s.results.ListByBatch(batchID)
	if err != nil {
		return nil, err
	}
	if len(all) == 0 {
		return nil, model.ErrNotFound
	}
	return all[0], nil // version DESC 已排序
}

// Compare 比较两个结果版本，返回核心指标差异。
func (s *Service) Compare(leftID, rightID string) (*Comparison, error) {
	l, err := s.results.Get(leftID)
	if err != nil {
		return nil, err
	}
	r, err := s.results.Get(rightID)
	if err != nil {
		return nil, err
	}
	if l.BatchID != r.BatchID {
		return nil, fmt.Errorf("%w: results from different batches", model.ErrInvalidArgument)
	}
	return &Comparison{
		Left:           l,
		Right:          r,
		MeanShiftDeg:   delta(l.Stats, r.Stats, func(x *model.CircularStats) float64 { return x.MeanDeg }),
		RLengthChange:  delta(l.Stats, r.Stats, func(x *model.CircularStats) float64 { return x.ResultantLength }),
		CountDelta:     r.ObservationCount - l.ObservationCount,
		BimodalChanged: l.Bimodality != nil && r.Bimodality != nil && l.Bimodality.IsBimodal != r.Bimodality.IsBimodal,
	}, nil
}

// Comparison 版本比较结果。
type Comparison struct {
	Left           *model.Result `json:"left"`
	Right          *model.Result `json:"right"`
	MeanShiftDeg   float64       `json:"mean_shift_deg"`
	RLengthChange  float64       `json:"r_length_change"`
	CountDelta     int           `json:"count_delta"`
	BimodalChanged bool          `json:"bimodal_changed"`
}

func delta(a, b *model.CircularStats, f func(*model.CircularStats) float64) float64 {
	if a == nil || b == nil {
		return 0
	}
	return f(b) - f(a)
}

func joinIDs(ids []string) string {
	out := ""
	for i, id := range ids {
		if i > 0 {
			out += ","
		}
		out += id
	}
	return out
}

func wrapInvalid(err error) error {
	if errors.Is(err, model.ErrInvalidState) {
		return err
	}
	return fmt.Errorf("%w: %v", model.ErrInvalidState, err)
}
