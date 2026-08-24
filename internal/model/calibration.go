package model

import (
	"errors"
	"fmt"
	"time"
)

// CalibrationStatus 校准版本状态机：draft → active → revoked
// 同一时刻至多一个生效（active）版本。
type CalibrationStatus string

const (
	CalibrationDraft   CalibrationStatus = "draft"
	CalibrationActive  CalibrationStatus = "active"
	CalibrationRevoked CalibrationStatus = "revoked"
)

// Calibration 一个仪器切片偏差校准版本。
// BiasDeg 表示把表观角度修正为真实取向所需的角度补偿（轴向，模 180°）。
type Calibration struct {
	ID          string            `json:"id"`
	BatchID     string            `json:"batch_id"`
	Status      CalibrationStatus `json:"status"`
	BiasDeg     float64           `json:"bias_deg"`
	SampleSize  int               `json:"sample_size"` // 用于估计偏差的观测数
	Confidence  float64           `json:"confidence"`  // 偏差估计置信度 [0,1]
	CreatedAt   time.Time         `json:"created_at"`
	ActivatedAt *time.Time        `json:"activated_at,omitempty"`
	RevokedAt   *time.Time        `json:"revoked_at,omitempty"`
}

// NewCalibration 创建校准草稿。
func NewCalibration(id, batchID string, biasDeg float64, sampleSize int, confidence float64) (*Calibration, error) {
	if id == "" {
		return nil, errors.New("calibration id required")
	}
	if batchID == "" {
		return nil, errors.New("calibration batch id required")
	}
	if biasDeg < 0 || biasDeg >= 180 {
		return nil, fmt.Errorf("calibration bias must be in [0,180), got %v", biasDeg)
	}
	if sampleSize < 1 {
		return nil, errors.New("calibration sample size must be >= 1")
	}
	if confidence < 0 || confidence > 1 {
		return nil, fmt.Errorf("calibration confidence out of range [0,1]: %v", confidence)
	}
	return &Calibration{
		ID:         id,
		BatchID:    batchID,
		Status:     CalibrationDraft,
		BiasDeg:    biasDeg,
		SampleSize: sampleSize,
		Confidence: confidence,
		CreatedAt:  now().UTC(),
	}, nil
}

// Activate 草稿 → 生效。
func (c *Calibration) Activate() error {
	if c.Status != CalibrationDraft {
		return fmt.Errorf("only draft calibration can be activated, current %s", c.Status)
	}
	c.Status = CalibrationActive
	t := now().UTC()
	c.ActivatedAt = &t
	c.RevokedAt = nil
	return nil
}

// Revoke 生效 → 废止。
func (c *Calibration) Revoke() error {
	if c.Status != CalibrationActive {
		return fmt.Errorf("only active calibration can be revoked, current %s", c.Status)
	}
	c.Status = CalibrationRevoked
	t := now().UTC()
	c.RevokedAt = &t
	return nil
}
