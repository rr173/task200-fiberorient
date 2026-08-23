// Package model 定义纸张纤维取向统计校准服务的领域实体、状态机与共享错误。
//
// 领域对象均为纯数据 + 不变式校验，不直接触碰数据库；持久化职责在 store 包，
// 业务编排在 service 层，HTTP 暴露在 httpapi 层。
package model

import (
	"errors"
	"fmt"
	"time"
)

// 时间函数可注入，便于测试与冒烟验证。
var now = time.Now

// BatchStatus 样品批次状态机：
// registered → observing → analyzable → published
// 任一非终态可被打回 observing 以补录观测。
type BatchStatus string

const (
	BatchRegistered BatchStatus = "registered"
	BatchObserving  BatchStatus = "observing"
	BatchAnalyzable BatchStatus = "analyzable"
	BatchPublished  BatchStatus = "published"
)

// ValidBatchTransitions 描述批次允许的合法流转。
var ValidBatchTransitions = map[BatchStatus][]BatchStatus{
	BatchRegistered: {BatchObserving},
	BatchObserving:  {BatchAnalyzable, BatchRegistered},
	BatchAnalyzable: {BatchPublished, BatchObserving},
	BatchPublished:  {BatchObserving}, // 补充观测后打回重算
}

// Batch 一个样品批次：登记切片方向，承载视野与角度观测。
type Batch struct {
	ID            string      `json:"id"`
	Name          string      `json:"name"`
	Material      string      `json:"material"`
	SliceAngleDeg float64     `json:"slice_angle_deg"` // 显微切片相对基准方向的角度
	Status        BatchStatus `json:"status"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
	Version       int         `json:"version"`
}

// NewBatch 校验并创建批次实体。
func NewBatch(id, name, material string, sliceAngleDeg float64) (*Batch, error) {
	if id == "" {
		return nil, errors.New("batch id required")
	}
	if name == "" {
		return nil, errors.New("batch name required")
	}
	if material == "" {
		return nil, errors.New("batch material required")
	}
	if sliceAngleDeg < 0 || sliceAngleDeg >= 180 {
		return nil, fmt.Errorf("slice angle must be in [0,180), got %v", sliceAngleDeg)
	}
	t := now().UTC()
	return &Batch{
		ID:            id,
		Name:          name,
		Material:      material,
		SliceAngleDeg: sliceAngleDeg,
		Status:        BatchRegistered,
		CreatedAt:     t,
		UpdatedAt:     t,
		Version:       1,
	}, nil
}

// CanTransition 返回从当前状态到目标状态是否合法。
func (b *Batch) CanTransition(target BatchStatus) bool {
	for _, s := range ValidBatchTransitions[b.Status] {
		if s == target {
			return true
		}
	}
	return false
}

// Transition 校验并推进状态机。
func (b *Batch) Transition(target BatchStatus) error {
	if !b.CanTransition(target) {
		return fmt.Errorf("invalid batch transition %s -> %s", b.Status, target)
	}
	b.Status = target
	b.UpdatedAt = now().UTC()
	b.Version++
	return nil
}

// UpdateSliceAngle 修改切片方向并打回观测中状态（需重新采集）。
func (b *Batch) UpdateSliceAngle(deg float64) error {
	if deg < 0 || deg >= 180 {
		return fmt.Errorf("slice angle must be in [0,180), got %v", deg)
	}
	b.SliceAngleDeg = deg
	b.Status = BatchObserving
	b.UpdatedAt = now().UTC()
	b.Version++
	return nil
}
