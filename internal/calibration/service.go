// Package calibration 校准模块：估计显微切片仪器偏差，管理校准版本
// （draft → active → revoked），同一批次至多一个生效版本。
package calibration

import (
	"errors"
	"fmt"
	"sync"

	"task200-fiberorient/internal/model"
	"task200-fiberorient/internal/statistics"
	"task200-fiberorient/internal/store"
)

// Service 校准模块服务。
type Service struct {
	cals   *store.CalibrationStore
	obs    *store.ObservationStore
	fields *store.FieldStore
	mu     sync.Mutex // 保护同一批次激活的串行性
}

// New 构造校准服务。
func New(cals *store.CalibrationStore, obs *store.ObservationStore, fields *store.FieldStore) *Service {
	return &Service{cals: cals, obs: obs, fields: fields}
}

// DraftResult 建草稿时的估计结果。
type DraftResult struct {
	Calibration *model.Calibration `json:"calibration"`
	BiasDeg     float64            `json:"bias_deg"`
	SampleSize  int                `json:"sample_size"`
	Confidence  float64            `json:"confidence"`
}

// EstimateAndDraft 基于双切片观测估计偏差并创建校准草稿。
// 需要同一批次同时存在 0° 与 90° 切片的有效观测（各至少 3 条）。
func (s *Service) EstimateAndDraft(batchID string, obsCount int) (*DraftResult, error) {
	fields, err := s.fields.ListByBatch(batchID)
	if err != nil {
		return nil, err
	}
	var (
		obs0  []float64
		obs90 []float64
	)
	for _, f := range fields {
		if f.Status != model.FieldValid {
			continue // 仅有效视野参与校准
		}
		obs, err := s.obs.ListByField(f.ID)
		if err != nil {
			return nil, err
		}
		for _, o := range obs {
			if f.SliceDeg == 0 {
				obs0 = append(obs0, o.AngleDeg)
			} else {
				obs90 = append(obs90, o.AngleDeg)
			}
		}
	}
	if len(obs0) < 3 || len(obs90) < 3 {
		return nil, fmt.Errorf("%w: calibration needs >=3 observations on each of 0/90 slices, got 0°=%d 90°=%d",
			model.ErrInsufficientData, len(obs0), len(obs90))
	}
	bias, conf := statistics.EstimateBias(obs0, obs90)
	id := fmt.Sprintf("cal-%s-%d", batchID, len(obs0)+len(obs90))
	c, err := model.NewCalibration(id, batchID, bias, len(obs0)+len(obs90), conf)
	if err != nil {
		return nil, err
	}
	if err := s.cals.Insert(c); err != nil {
		return nil, err
	}
	return &DraftResult{Calibration: c, BiasDeg: bias, SampleSize: len(obs0) + len(obs90), Confidence: conf}, nil
}

// Activate 激活草稿：先废止同批次旧生效版本，再置新版本生效。
// 同一批次串行（互斥锁），防止并发激活产生两个 active。
func (s *Service) Activate(id string) (*model.Calibration, error) {
	c, err := s.cals.Get(id)
	if err != nil {
		return nil, err
	}
	if c.Status != model.CalibrationDraft {
		return nil, fmt.Errorf("%w: calibration not draft", model.ErrInvalidState)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.cals.RevokeAllActive(""); err != nil {
		return nil, err
	}
	if err := c.Activate(); err != nil {
		return nil, err
	}
	if err := s.cals.Update(c); err != nil {
		return nil, err
	}
	return c, nil
}

// Revoke 废止生效版本。
func (s *Service) Revoke(id string) (*model.Calibration, error) {
	c, err := s.cals.Get(id)
	if err != nil {
		return nil, err
	}
	if c.Status != model.CalibrationActive {
		return nil, fmt.Errorf("%w: calibration not active", model.ErrInvalidState)
	}
	if err := c.Revoke(); err != nil {
		return nil, err
	}
	if err := s.cals.Update(c); err != nil {
		return nil, err
	}
	return c, nil
}

// Get 取校准版本详情。
func (s *Service) Get(id string) (*model.Calibration, error) {
	return s.cals.Get(id)
}

// ListByBatch 列出批次校准版本。
func (s *Service) ListByBatch(batchID string) ([]*model.Calibration, error) {
	return s.cals.ListByBatch(batchID)
}

// Active 取批次当前生效校准版本。
func (s *Service) Active(batchID string) (*model.Calibration, error) {
	c, err := s.cals.Active(batchID)
	if errors.Is(err, model.ErrNotFound) {
		return nil, fmt.Errorf("%w: no active calibration for batch %s", model.ErrInsufficientData, batchID)
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}
