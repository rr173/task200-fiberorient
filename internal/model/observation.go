package model

import (
	"errors"
	"fmt"
	"time"
)

// AngleUnit 角度单位：拒绝单位混用（同一导入批次内必须统一）。
type AngleUnit string

const (
	UnitDegrees AngleUnit = "deg"
	UnitRadians AngleUnit = "rad"
)

// Observation 一条纤维角度观测记录。
// 纤维取向是轴向数据（轴向 = 无向直线），角度以 180° 为模，0° 与 180° 等价。
type Observation struct {
	ID          string    `json:"id"`
	BatchID     string    `json:"batch_id"`
	FieldID     string    `json:"field_id"`
	AngleDeg    float64   `json:"angle_deg"`   // 归一化到 [0,180)
	Unit        AngleUnit `json:"unit"`        // 原始提交单位（保留以支持单位审计）
	Fingerprint string    `json:"fingerprint"` // 观测指纹（幂等去重）
	CreatedAt   time.Time `json:"created_at"`
}

// NewObservation 校验并创建观测。
// submissionID 标识一次批量提交（同一次导入的重复重试幂等）；seq 为该提交内序号。
// angle 的归一化：若 unit=deg，先 mod 180 映射到 [0,180)；若 unit=rad，转 deg 后再归一化。
func NewObservation(id, batchID, fieldID string, angle float64, unit AngleUnit, submissionID string, seq int) (*Observation, error) {
	if id == "" {
		return nil, errors.New("observation id required")
	}
	if batchID == "" {
		return nil, errors.New("observation batch id required")
	}
	if fieldID == "" {
		return nil, errors.New("observation field id required")
	}
	if submissionID == "" {
		return nil, errors.New("observation submission id required")
	}
	deg, err := normalizeAngle(angle, unit)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidArgument, err)
	}
	return &Observation{
		ID:          id,
		BatchID:     batchID,
		FieldID:     fieldID,
		AngleDeg:    deg,
		Unit:        unit,
		Fingerprint: fingerprintOf(batchID, fieldID, submissionID, seq),
		CreatedAt:   now().UTC(),
	}, nil
}

// normalizeAngle 将任意角度归一化到 [0,180)。
func normalizeAngle(angle float64, unit AngleUnit) (float64, error) {
	switch unit {
	case UnitDegrees:
		if angle < 0 || angle > 360 {
			return 0, fmt.Errorf("degree angle out of range [0,360]: %v", angle)
		}
	case UnitRadians:
		if angle < 0 || angle > 2*3.141592653589793 {
			return 0, fmt.Errorf("radian angle out of range [0,2π]: %v", angle)
		}
		angle = angle * 180 / 3.141592653589793
	default:
		return 0, fmt.Errorf("unsupported angle unit %q", unit)
	}
	deg := angle
	for deg >= 180 {
		deg -= 180
	}
	if deg < 0 {
		deg += 180
	}
	return deg, nil
}

// fingerprintOf 计算观测指纹：批次+视野+提交批次+序号（同一次提交重试幂等，
// 同角度多条纤维不误判重复）。
//
// 指纹包含 fieldID：同一提交标识可分别导入同一批次的两个不同切片视野，
// 二者属于不同视野而各自保留；仅当同一视野内同提交同序号重试时才判为重复。
func fingerprintOf(batchID, fieldID, submissionID string, seq int) string {
	return fmt.Sprintf("%s|%s|%s|%d", batchID, fieldID, submissionID, seq)
}

// FingerprintValue 返回观测指纹。
func (o *Observation) FingerprintValue() string {
	return o.Fingerprint
}
