// Package observation 观测模块：负责纤维角度观测的登记、批量导入（指纹幂等）
// 与单位一致性校验。
package observation

import (
	"errors"
	"fmt"

	"task200-fiberorient/internal/model"
	"task200-fiberorient/internal/store"
)

// Service 观测模块服务。
type Service struct {
	obs *store.ObservationStore
	fld *store.FieldStore
}

// New 构造观测服务。
func New(obs *store.ObservationStore, fld *store.FieldStore) *Service {
	return &Service{obs: obs, fld: fld}
}

// ImportResult 一次导入的结果。
type ImportResult struct {
	Inserted   int      `json:"inserted"`
	Skipped    int      `json:"skipped"` // 指纹幂等跳过
	Rejected   int      `json:"rejected"`
	FieldID    string   `json:"field_id"`
	Unit       string   `json:"unit"`
	Rejections []string `json:"rejections,omitempty"`
}

// Import 批量导入角度观测。同一批必须单位统一；同指纹观测幂等跳过。
// submissionID 标识本次提交（重试幂等），field 必须存在且属于 batch，
// 且处于 pending/valid/polluted 状态（不可导入到已剔除视野）。
func (s *Service) Import(batchID, fieldID string, unit model.AngleUnit, angles []float64, submissionID string) (*ImportResult, error) {
	if len(angles) == 0 {
		return nil, fmt.Errorf("%w: empty angle list", model.ErrBadInput)
	}
	if submissionID == "" {
		return nil, fmt.Errorf("%w: submission id required", model.ErrInvalidArgument)
	}
	if unit != model.UnitDegrees && unit != model.UnitRadians {
		return nil, fmt.Errorf("%w: unsupported unit %q", model.ErrInvalidArgument, unit)
	}
	f, err := s.fld.Get(fieldID)
	if errors.Is(err, model.ErrNotFound) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if f.BatchID != batchID {
		return nil, fmt.Errorf("%w: field %s not in batch %s", model.ErrInvalidArgument, fieldID, batchID)
	}
	if f.Status == model.FieldExcluded {
		return nil, fmt.Errorf("%w: cannot import into excluded field", model.ErrInvalidState)
	}

	res := &ImportResult{FieldID: fieldID, Unit: string(unit)}
	for i, a := range angles {
		// id 包含 fieldID：同一提交标识可分别导入同一批次的两个不同视野，
		// 二者各自的观测 id 互不冲突（跨视野导入各自保留）。
		id := fmt.Sprintf("obs-%s-%s-%d", fieldID, submissionID, i)
		o, err := model.NewObservation(id, batchID, fieldID, a, unit, submissionID, i)
		if err != nil {
			res.Rejected++
			res.Rejections = append(res.Rejections, fmt.Sprintf("#%d: %v", i, err))
			continue
		}
		ok, err := s.obs.Insert(o)
		if err != nil {
			return res, err
		}
		if ok {
			res.Inserted++
		} else {
			res.Skipped++
		}
	}
	if res.Rejected > 0 {
		return nil, fmt.Errorf("%w: %d observations rejected", model.ErrBadInput, res.Rejected)
	}
	return res, nil
}

// ListByBatch 列出批次观测。
func (s *Service) ListByBatch(batchID string) ([]*model.Observation, error) {
	return s.obs.ListByBatch(batchID)
}

// CountByBatch 批次观测总数。
func (s *Service) CountByBatch(batchID string) (int, error) {
	return s.obs.CountByBatch(batchID)
}

// DistinctUnits 返回批次内出现过的单位（用于单位混用审计）。
func (s *Service) DistinctUnits(batchID string) ([]model.AngleUnit, error) {
	return s.obs.DistinctUnits(batchID)
}
