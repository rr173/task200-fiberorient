package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"task200-fiberorient/internal/model"
)

// ObservationStore 观测持久化。
type ObservationStore struct{ db *DB }

// NewObservationStore 构造观测仓储。
func NewObservationStore(db *DB) *ObservationStore { return &ObservationStore{db: db} }

// Insert 插入观测；同指纹冲突时返回 (false, nil) 表示幂等跳过。
func (s *ObservationStore) Insert(o *model.Observation) (bool, error) {
	_, err := s.db.SQL().Exec(
		`INSERT INTO observations (id, batch_id, field_id, angle_deg, unit, fingerprint, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		o.ID, o.BatchID, o.FieldID, o.AngleDeg, string(o.Unit), o.Fingerprint, ts(o.CreatedAt),
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return false, nil
		}
		return false, fmt.Errorf("insert observation: %w", err)
	}
	return true, nil
}

// Get 按 ID 取观测。
func (s *ObservationStore) Get(id string) (*model.Observation, error) {
	row := s.db.SQL().QueryRow(
		`SELECT id, batch_id, field_id, angle_deg, unit, fingerprint, created_at
		 FROM observations WHERE id = ?`, id)
	o, err := scanObservation(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return o, nil
}

// ListByField 列出视野下观测。
func (s *ObservationStore) ListByField(fieldID string) ([]*model.Observation, error) {
	rows, err := s.db.SQL().Query(
		`SELECT id, batch_id, field_id, angle_deg, unit, fingerprint, created_at
		 FROM observations WHERE field_id = ? ORDER BY created_at ASC, id ASC`, fieldID)
	if err != nil {
		return nil, fmt.Errorf("list observations by field: %w", err)
	}
	defer rows.Close()
	return scanObservations(rows)
}

// ListByBatch 列出批次下观测。
func (s *ObservationStore) ListByBatch(batchID string) ([]*model.Observation, error) {
	rows, err := s.db.SQL().Query(
		`SELECT id, batch_id, field_id, angle_deg, unit, fingerprint, created_at
		 FROM observations WHERE batch_id = ? ORDER BY created_at ASC, id ASC`, batchID)
	if err != nil {
		return nil, fmt.Errorf("list observations by batch: %w", err)
	}
	defer rows.Close()
	return scanObservations(rows)
}

// ListByBatchFields 列出批次下指定视野集合的观测。
func (s *ObservationStore) ListByBatchFields(batchID string, fieldIDs []string) ([]*model.Observation, error) {
	if len(fieldIDs) == 0 {
		return nil, nil
	}
	placeholders := strings.Repeat("?,", len(fieldIDs))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, 0, len(fieldIDs)+1)
	args = append(args, batchID)
	for _, id := range fieldIDs {
		args = append(args, id)
	}
	rows, err := s.db.SQL().Query(
		`SELECT id, batch_id, field_id, angle_deg, unit, fingerprint, created_at
		 FROM observations WHERE batch_id = ? AND field_id IN (`+placeholders+`)
		 ORDER BY created_at ASC, id ASC`, args...)
	if err != nil {
		return nil, fmt.Errorf("list observations by batch fields: %w", err)
	}
	defer rows.Close()
	return scanObservations(rows)
}

// CountByBatch 批次观测总数。
func (s *ObservationStore) CountByBatch(batchID string) (int, error) {
	var n int
	if err := s.db.SQL().QueryRow(
		`SELECT COUNT(*) FROM observations WHERE batch_id=?`, batchID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count observations: %w", err)
	}
	return n, nil
}

// DistinctUnits 返回批次内出现的单位集合（用于单位混用检测）。
func (s *ObservationStore) DistinctUnits(batchID string) ([]model.AngleUnit, error) {
	rows, err := s.db.SQL().Query(
		`SELECT DISTINCT unit FROM observations WHERE batch_id=?`, batchID)
	if err != nil {
		return nil, fmt.Errorf("distinct units: %w", err)
	}
	defer rows.Close()
	var units []model.AngleUnit
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return nil, err
		}
		units = append(units, model.AngleUnit(u))
	}
	return units, rows.Err()
}

func scanObservations(rows *sql.Rows) ([]*model.Observation, error) {
	var out []*model.Observation
	for rows.Next() {
		o, err := scanObservation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func scanObservation(sc scanner) (*model.Observation, error) {
	var (
		o          model.Observation
		unit       string
		createdRaw string
	)
	if err := sc.Scan(&o.ID, &o.BatchID, &o.FieldID, &o.AngleDeg, &unit, &o.Fingerprint, &createdRaw); err != nil {
		return nil, err
	}
	o.Unit = model.AngleUnit(unit)
	var err error
	o.CreatedAt, err = parseTS(createdRaw)
	if err != nil {
		return nil, err
	}
	return &o, nil
}
