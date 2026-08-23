// Package service 编排层：组装各业务模块并暴露给 httpapi 与 main。
package service

import (
	"errors"
	"fmt"
	"sync"

	"task200-fiberorient/internal/calibration"
	"task200-fiberorient/internal/cleaning"
	"task200-fiberorient/internal/model"
	"task200-fiberorient/internal/observation"
	"task200-fiberorient/internal/result"
	"task200-fiberorient/internal/store"
)

// App 应用编排根：聚合全部模块服务。
type App struct {
	db *store.DB

	Batches *BatchService
	Fields  *cleaning.Service
	Obs     *observation.Service
	Cals    *calibration.Service
	Results *result.Service

	mu sync.Mutex // 批次状态流转串行
}

// New 组装应用服务。
func New(db *store.DB) (*App, error) {
	bs := store.NewBatchStore(db)
	fs := store.NewFieldStore(db)
	os := store.NewObservationStore(db)
	cs := store.NewCalibrationStore(db)
	rs := store.NewResultStore(db)

	return &App{
		db:      db,
		Batches: NewBatchService(bs),
		Fields:  cleaning.New(fs),
		Obs:     observation.New(os, fs),
		Cals:    calibration.New(cs, os, fs),
		Results: result.New(rs, cs, os, fs),
	}, nil
}

// DB 暴露底层连接（自检用）。
func (a *App) DB() *store.DB { return a.db }

// BatchService 批次编排：管理批次生命周期并协调视野/观测。
type BatchService struct {
	batches *store.BatchStore
}

// NewBatchService 构造批次服务。
func NewBatchService(bs *store.BatchStore) *BatchService { return &BatchService{batches: bs} }

// Create 创建批次。
func (s *BatchService) Create(id, name, material string, sliceAngleDeg float64) (*model.Batch, error) {
	b, err := model.NewBatch(id, name, material, sliceAngleDeg)
	if err != nil {
		return nil, err
	}
	if err := s.batches.Insert(b); err != nil {
		return nil, err
	}
	return b, nil
}

// Get 取批次详情。
func (s *BatchService) Get(id string) (*model.Batch, error) {
	return s.batches.Get(id)
}

// List 列出批次。
func (s *BatchService) List() ([]*model.Batch, error) {
	return s.batches.List()
}

// Transition 推进批次状态。
func (s *BatchService) Transition(id string, target model.BatchStatus) (*model.Batch, error) {
	b, err := s.batches.Get(id)
	if err != nil {
		return nil, err
	}
	if err := b.Transition(target); err != nil {
		return nil, fmt.Errorf("%w: %v", model.ErrInvalidState, err)
	}
	b.Version--
	if err := s.batches.Update(b); err != nil {
		return nil, err
	}
	return b, nil
}

// UpdateSliceAngle 修改切片方向（自动打回 observing）。
func (s *BatchService) UpdateSliceAngle(id string, deg float64) (*model.Batch, error) {
	b, err := s.batches.Get(id)
	if err != nil {
		return nil, err
	}
	if err := b.UpdateSliceAngle(deg); err != nil {
		return nil, err
	}
	b.Version -= 2
	if err := s.batches.Update(b); err != nil {
		return nil, err
	}
	return b, nil
}

// Publish 发布批次：要求批次可分析（analyzable）且有已发布结果。
func (a *App) Publish(batchID string) (*model.Batch, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	b, err := a.Batches.Get(batchID)
	if err != nil {
		return nil, err
	}
	if b.Status != model.BatchAnalyzable {
		return nil, fmt.Errorf("%w: batch must be analyzable to publish, current %s", model.ErrInvalidState, b.Status)
	}
	latest, err := a.Results.Latest(batchID)
	if errors.Is(err, model.ErrNotFound) {
		return nil, fmt.Errorf("%w: no results to publish", model.ErrInsufficientData)
	}
	if err != nil {
		return nil, err
	}
	if latest.Status != model.ResultFrozen {
		return nil, fmt.Errorf("%w: latest result not frozen", model.ErrInvalidState)
	}
	return a.Batches.Transition(batchID, model.BatchPublished)
}

// FullStatSnapshot 组合"视野筛选 + 有效视野"供统计调用（编排层职责）。
func (a *App) FullStatSnapshot(batchID string) ([]string, string, error) {
	fields, err := a.Fields.EffectiveFields(batchID)
	if err != nil {
		return nil, "", err
	}
	if len(fields) == 0 {
		return nil, "", fmt.Errorf("%w: no valid fields in batch %s", model.ErrInsufficientData, batchID)
	}
	ids := make([]string, 0, len(fields))
	snap := ""
	for i, f := range fields {
		ids = append(ids, f.ID)
		if i > 0 {
			snap += ","
		}
		snap += f.ID
	}
	return ids, snap, nil
}
