// Package cleaning 清洗模块：管理视野状态（待清洗/有效/污染/已剔除），
// 支持污染剔除与恢复，是统计入口的数据闸门。
package cleaning

import (
	"errors"
	"fmt"

	"task200-fiberorient/internal/model"
	"task200-fiberorient/internal/store"
)

// Service 清洗模块服务。
type Service struct {
	fields *store.FieldStore
}

// New 构造清洗服务。
func New(fields *store.FieldStore) *Service { return &Service{fields: fields} }

// Register 登记一个待清洗视野（sliceDeg ∈ {0,90}）。
func (s *Service) Register(batchID, label string, sliceDeg float64) (*model.Field, error) {
	id := fmt.Sprintf("fld-%s-%d", batchID, len(label)+len(batchID))
	// 生成稳定 ID：batch 内序号由 store 查询决定，这里用简单唯一值。
	f, err := model.NewField(id, batchID, label, sliceDeg)
	if err != nil {
		return nil, err
	}
	if err := s.fields.Insert(f); err != nil {
		return nil, err
	}
	return f, nil
}

// RegisterWithID 用显式 ID 登记视野（避免与并发导入的 ID 冲突，供内部/测试使用）。
func (s *Service) RegisterWithID(id, batchID, label string, sliceDeg float64) (*model.Field, error) {
	f, err := model.NewField(id, batchID, label, sliceDeg)
	if err != nil {
		return nil, err
	}
	if err := s.fields.Insert(f); err != nil {
		return nil, err
	}
	return f, nil
}

// ListByBatch 列出批次视野。
func (s *Service) ListByBatch(batchID string) ([]*model.Field, error) {
	return s.fields.ListByBatch(batchID)
}

// Get 取视野详情。
func (s *Service) Get(id string) (*model.Field, error) {
	return s.fields.Get(id)
}

// MarkValid 标记视野有效。
func (s *Service) MarkValid(id string) (*model.Field, error) {
	f, err := s.fields.Get(id)
	if err != nil {
		return nil, err
	}
	if err := f.MarkValid(); err != nil {
		return nil, wrapState(err)
	}
	if err := s.fields.Update(f); err != nil {
		return nil, err
	}
	return f, nil
}

// MarkPolluted 标记视野污染（需原因）。
func (s *Service) MarkPolluted(id, reason string) (*model.Field, error) {
	if reason == "" {
		return nil, fmt.Errorf("%w: pollute reason required", model.ErrInvalidArgument)
	}
	f, err := s.fields.Get(id)
	if err != nil {
		return nil, err
	}
	if err := f.MarkPolluted(reason); err != nil {
		return nil, wrapState(err)
	}
	if err := s.fields.Update(f); err != nil {
		return nil, err
	}
	return f, nil
}

// Exclude 剔除视野（终态）。
func (s *Service) Exclude(id string) (*model.Field, error) {
	f, err := s.fields.Get(id)
	if err != nil {
		return nil, err
	}
	if err := f.Exclude(); err != nil {
		return nil, wrapState(err)
	}
	if err := s.fields.Update(f); err != nil {
		return nil, err
	}
	return f, nil
}

// EffectiveFields 返回批次下全部有效视野。
// 仅 valid 视野参与统计：污染（polluted）可恢复，已剔除（excluded）为终态，
// 二者都不应进入有效视野快照或观测数量，否则清洗结果与统计输入不一致。
func (s *Service) EffectiveFields(batchID string) ([]*model.Field, error) {
	all, err := s.fields.ListByBatch(batchID)
	if err != nil {
		return nil, err
	}
	var out []*model.Field
	for _, f := range all {
		if f.Status == model.FieldValid {
			out = append(out, f)
		}
	}
	return out, nil
}

// Snapshot 生成有效视野 ID 快照（用于结果绑定）。
func (s *Service) Snapshot(batchID string) (string, error) {
	return s.fields.FieldSnapshot(batchID)
}

func wrapState(err error) error {
	if errors.Is(err, model.ErrInvalidState) {
		return err
	}
	// model.Transition 返回的是普通 error，统一映射。
	return fmt.Errorf("%w: %v", model.ErrInvalidState, err)
}
