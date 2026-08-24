package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"task200-fiberorient/internal/model"
)

// BatchStore 批次持久化。
type BatchStore struct{ db *DB }

// NewBatchStore 构造批次仓储。
func NewBatchStore(db *DB) *BatchStore { return &BatchStore{db: db} }

// Insert 插入批次。
func (s *BatchStore) Insert(b *model.Batch) error {
	_, err := s.db.SQL().Exec(
		`INSERT INTO batches (id, name, material, slice_angle, status, version, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		b.ID, b.Name, b.Material, b.SliceAngleDeg, string(b.Status), b.Version,
		ts(b.CreatedAt), ts(b.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("insert batch: %w", err)
	}
	return nil
}

// Get 按 ID 取批次。
func (s *BatchStore) Get(id string) (*model.Batch, error) {
	row := s.db.SQL().QueryRow(
		`SELECT id, name, material, slice_angle, status, version, created_at, updated_at
		 FROM batches WHERE id = ?`, id)
	b, err := scanBatch(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return b, nil
}

// List 列出全部批次（按创建时间倒序）。
func (s *BatchStore) List() ([]*model.Batch, error) {
	rows, err := s.db.SQL().Query(
		`SELECT id, name, material, slice_angle, status, version, created_at, updated_at
		 FROM batches ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list batches: %w", err)
	}
	defer rows.Close()
	var out []*model.Batch
	for rows.Next() {
		b, err := scanBatch(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// Update 更新批次（乐观锁：version 必须匹配）。
func (s *BatchStore) Update(b *model.Batch) error {
	res, err := s.db.SQL().Exec(
		`UPDATE batches SET name=?, material=?, slice_angle=?, status=?, version=?, updated_at=?
		 WHERE id=? AND version=?`,
		b.Name, b.Material, b.SliceAngleDeg, string(b.Status), b.Version, ts(b.UpdatedAt),
		b.ID, b.Version-1,
	)
	if err != nil {
		return fmt.Errorf("update batch: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.ErrConflict
	}
	return nil
}

// Count 批次总数（自检用）。
func (s *BatchStore) Count() (int, error) {
	var n int
	if err := s.db.SQL().QueryRow(`SELECT COUNT(*) FROM batches`).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

type scanner interface{ Scan(dest ...any) error }

func scanBatch(sc scanner) (*model.Batch, error) {
	var (
		b          model.Batch
		status     string
		createdRaw string
		updatedRaw string
	)
	if err := sc.Scan(&b.ID, &b.Name, &b.Material, &b.SliceAngleDeg, &status, &b.Version,
		&createdRaw, &updatedRaw); err != nil {
		return nil, err
	}
	b.Status = model.BatchStatus(status)
	var err error
	b.CreatedAt, err = parseTS(createdRaw)
	if err != nil {
		return nil, err
	}
	b.UpdatedAt, err = parseTS(updatedRaw)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func ts(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func parseTS(s string) (time.Time, error) { return time.Parse(time.RFC3339Nano, s) }
