package model

import (
	"errors"
	"fmt"
	"time"
)

// ResultStatus 统计结果状态机：
// computing → insufficient_confidence / publishable → frozen
// 冻结（frozen）为终态；新增观测会创建替代版本（新结果记录）。
type ResultStatus string

const (
	ResultComputing        ResultStatus = "computing"
	ResultInsufficientConf ResultStatus = "insufficient_confidence"
	ResultPublishable      ResultStatus = "publishable"
	ResultFrozen           ResultStatus = "frozen"
)

// CircularStats 圆周统计量（纤维取向轴向统计）。
type CircularStats struct {
	MeanDeg          float64 `json:"mean_deg"`          // 圆周均值（轴向，模 180）
	ResultantLength  float64 `json:"resultant_length"`  // 合矢量长度 R ∈ [0,1]
	CircularVariance float64 `json:"circular_variance"` // 1 - R
	SampleSize       int     `json:"sample_size"`
	Kappa            float64 `json:"kappa"`      // von Mises 集中度估计
	RayleighP        float64 `json:"rayleigh_p"` // Rayleigh 检验 p 值（各向同性原假设）
}

// Bimodality 双峰检测结果。
type Bimodality struct {
	IsBimodal     bool    `json:"is_bimodal"`
	PrimaryDeg    float64 `json:"primary_deg"`
	SecondaryDeg  float64 `json:"secondary_deg"`
	DipStatistic  float64 `json:"dip_statistic"`
	SeparationDeg float64 `json:"separation_deg"` // 双峰主轴间距
}

// ConfidenceInterval 均值置信区间。
type ConfidenceInterval struct {
	LowerDeg float64 `json:"lower_deg"`
	UpperDeg float64 `json:"upper_deg"`
	Level    float64 `json:"level"`
	Method   string  `json:"method"`
}

// Result 一次统计结果版本（绑定输入筛选快照与校准版本）。
type Result struct {
	ID               string              `json:"id"`
	BatchID          string              `json:"batch_id"`
	Version          int                 `json:"version"`
	Status           ResultStatus        `json:"status"`
	CalibrationID    string              `json:"calibration_id"` // 绑定的校准版本
	FieldSnapshot    string              `json:"field_snapshot"` // 参与统计的视野快照（指纹）
	ObservationCount int                 `json:"observation_count"`
	Stats            *CircularStats      `json:"stats,omitempty"`
	Bimodality       *Bimodality         `json:"bimodality,omitempty"`
	Confidence       *ConfidenceInterval `json:"confidence,omitempty"`
	CreatedAt        time.Time           `json:"created_at"`
	FrozenAt         *time.Time          `json:"frozen_at,omitempty"`
}

// NewResult 创建计算中的结果版本。
func NewResult(id, batchID string, version int, calibrationID, fieldSnapshot string, obsCount int) (*Result, error) {
	if id == "" {
		return nil, errors.New("result id required")
	}
	if batchID == "" {
		return nil, errors.New("result batch id required")
	}
	if version < 1 {
		return nil, fmt.Errorf("result version must be >= 1, got %d", version)
	}
	if calibrationID == "" {
		return nil, errors.New("result calibration id required")
	}
	if obsCount < 0 {
		return nil, fmt.Errorf("observation count must be >= 0, got %d", obsCount)
	}
	return &Result{
		ID:               id,
		BatchID:          batchID,
		Version:          version,
		Status:           ResultComputing,
		CalibrationID:    calibrationID,
		FieldSnapshot:    fieldSnapshot,
		ObservationCount: obsCount,
		CreatedAt:        now().UTC(),
	}, nil
}

// Finalize 计算完成：根据置信度落到 publishable 或 insufficient_confidence。
func (r *Result) Finalize(stats *CircularStats, bim *Bimodality, ci *ConfidenceInterval, minConf float64) error {
	if r.Status != ResultComputing {
		return fmt.Errorf("only computing result can be finalized, current %s", r.Status)
	}
	r.Stats = stats
	r.Bimodality = bim
	r.Confidence = ci
	if ci == nil || ci.UpperDeg-ci.LowerDeg <= minConf {
		r.Status = ResultPublishable
	} else {
		r.Status = ResultInsufficientConf
	}
	return nil
}

// Freeze 冻结结果（终态，禁止直接编辑）。
func (r *Result) Freeze() error {
	if r.Status != ResultPublishable {
		return fmt.Errorf("only publishable result can be frozen, current %s", r.Status)
	}
	r.Status = ResultFrozen
	r.FrozenAt = nil
	return nil
}
