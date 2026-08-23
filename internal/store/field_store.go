package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"task200-fiberorient/internal/model"
)

// FieldStore 视野持久化。
type FieldStore struct{ db *DB }

// NewFieldStore 构造视野仓储。
func NewFieldStore(db *DB) *FieldStore { return &FieldStore{db: db} }

// Insert 插入视野。
func (s *FieldStore) Insert(f *model.Field) error {
	_, err := s.db.SQL().Exec(
		`INSERT INTO fields (id, batch_id, label, slice_deg, status, pollute_by, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		f.ID, f.BatchID, f.Label, f.SliceDeg, string(f.Status), f.PolluteBy, ts(f.CreatedAt), ts(f.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("insert field: %w", err)
	}
	return nil
}

// Get 按 ID 取视野。
func (s *FieldStore) Get(id string) (*model.Field, error) {
	row := s.db.SQL().QueryRow(
		`SELECT id, batch_id, label, slice_deg, status, pollute_by, created_at, updated_at
		 FROM fields WHERE id = ?`, id)
	f, err := scanField(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return f, nil
}

// ListByBatch 列出批次下全部视野。
func (s *FieldStore) ListByBatch(batchID string) ([]*model.Field, error) {
	rows, err := s.db.SQL().Query(
		`SELECT id, batch_id, label, slice_deg, status, pollute_by, created_at, updated_at
		 FROM fields WHERE batch_id = ? ORDER BY created_at ASC`, batchID)
	if err != nil {
		return nil, fmt.Errorf("list fields: %w", err)
	}
	defer rows.Close()
	var out []*model.Field
	for rows.Next() {
		f, err := scanField(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// Update 更新视野。
func (s *FieldStore) Update(f *model.Field) error {
	_, err := s.db.SQL().Exec(
		`UPDATE fields SET label=?, slice_deg=?, status=?, pollute_by=?, updated_at=? WHERE id=?`,
		f.Label, f.SliceDeg, string(f.Status), f.PolluteBy, ts(f.UpdatedAt), f.ID,
	)
	if err != nil {
		return fmt.Errorf("update field: %w", err)
	}
	return nil
}

// CountValid 统计批次下有效视野数。
func (s *FieldStore) CountValid(batchID string) (int, error) {
	var n int
	err := s.db.SQL().QueryRow(
		`SELECT COUNT(*) FROM fields WHERE batch_id=? AND status=?`, batchID, string(model.FieldValid),
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count valid fields: %w", err)
	}
	return n, nil
}

// FieldSnapshot 返回参与统计的视野 ID 快照（仅有效，排序稳定）。
func (s *FieldStore) FieldSnapshot(batchID string) (string, error) {
	rows, err := s.db.SQL().Query(
		`SELECT id FROM fields WHERE batch_id=? AND status=? ORDER BY id ASC`, batchID, string(model.FieldValid))
	if err != nil {
		return "", fmt.Errorf("field snapshot: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return "", err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if len(ids) == 0 {
		return "", model.ErrInsufficientData
	}
	snapshot := ""
	for i, id := range ids {
		if i > 0 {
			snapshot += ","
		}
		snapshot += id
	}
	return snapshot, nil
}

func scanField(sc scanner) (*model.Field, error) {
	var (
		f          model.Field
		status     string
		createdRaw string
		updatedRaw string
	)
	if err := sc.Scan(&f.ID, &f.BatchID, &f.Label, &f.SliceDeg, &status, &f.PolluteBy, &createdRaw, &updatedRaw); err != nil {
		return nil, err
	}
	f.Status = model.FieldStatus(status)
	var err error
	f.CreatedAt, err = parseTS(createdRaw)
	if err != nil {
		return nil, err
	}
	f.UpdatedAt, err = parseTS(updatedRaw)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// tsNull 处理可空时间。
func tsNull(t *time.Time) any {
	if t == nil {
		return nil
	}
	return ts(*t)
}
