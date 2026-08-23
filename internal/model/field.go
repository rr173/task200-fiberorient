package model

import (
	"errors"
	"fmt"
	"time"
)

// FieldStatus 视野状态机：
// pending → valid / polluted → excluded
// 已剔除（excluded）是终态，但污染（polluted）可恢复为有效（valid）。
type FieldStatus string

const (
	FieldPending  FieldStatus = "pending"
	FieldValid    FieldStatus = "valid"
	FieldPolluted FieldStatus = "polluted"
	FieldExcluded FieldStatus = "excluded"
)

// ValidFieldTransitions 视野合法流转。
var ValidFieldTransitions = map[FieldStatus][]FieldStatus{
	FieldPending:  {FieldValid, FieldPolluted, FieldExcluded},
	FieldValid:    {FieldPolluted, FieldExcluded},
	FieldPolluted: {FieldValid, FieldExcluded},
	FieldExcluded: {}, // 终态
}

// Field 显微切片上的一个视野：容纳若干角度观测。
// SliceDeg 为视野所属切片的基准方向（0 或 90），用于双切片偏差校准。
type Field struct {
	ID        string      `json:"id"`
	BatchID   string      `json:"batch_id"`
	Label     string      `json:"label"`
	SliceDeg  float64     `json:"slice_deg"`
	Status    FieldStatus `json:"status"`
	PolluteBy string      `json:"pollute_by,omitempty"` // 污染原因（脏污/撕裂/析出覆盖）
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// NewField 创建待清洗视野（sliceDeg ∈ {0, 90}）。
func NewField(id, batchID, label string, sliceDeg float64) (*Field, error) {
	if id == "" {
		return nil, errors.New("field id required")
	}
	if batchID == "" {
		return nil, errors.New("field batch id required")
	}
	if label == "" {
		return nil, errors.New("field label required")
	}
	if sliceDeg != 0 && sliceDeg != 90 {
		return nil, fmt.Errorf("field slice deg must be 0 or 90, got %v", sliceDeg)
	}
	t := now().UTC()
	return &Field{
		ID:        id,
		BatchID:   batchID,
		Label:     label,
		SliceDeg:  sliceDeg,
		Status:    FieldPending,
		CreatedAt: t,
		UpdatedAt: t,
	}, nil
}

// CanTransition 校验流转合法性。
func (f *Field) CanTransition(target FieldStatus) bool {
	for _, s := range ValidFieldTransitions[f.Status] {
		if s == target {
			return true
		}
	}
	return false
}

// Transition 通用流转。
func (f *Field) Transition(target FieldStatus) error {
	if !f.CanTransition(target) {
		return fmt.Errorf("invalid field transition %s -> %s", f.Status, target)
	}
	f.Status = target
	f.UpdatedAt = now().UTC()
	return nil
}

// MarkValid 标记为有效。
func (f *Field) MarkValid() error {
	if err := f.Transition(FieldValid); err != nil {
		return err
	}
	f.PolluteBy = ""
	return nil
}

// MarkPolluted 标记为污染（需给出原因）。
func (f *Field) MarkPolluted(reason string) error {
	if reason == "" {
		return errors.New("pollute reason required")
	}
	if err := f.Transition(FieldPolluted); err != nil {
		return err
	}
	f.PolluteBy = reason
	return nil
}

// Exclude 剔除视野（终态）。
func (f *Field) Exclude() error {
	return f.Transition(FieldExcluded)
}
