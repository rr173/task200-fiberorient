package store

import (
	"database/sql"
	"errors"
	"fmt"

	"task200-fiberorient/internal/model"
)

// CalibrationStore 校准版本持久化。
type CalibrationStore struct{ db *DB }

// NewCalibrationStore 构造校准仓储。
func NewCalibrationStore(db *DB) *CalibrationStore { return &CalibrationStore{db: db} }

// Insert 插入校准版本。
func (s *CalibrationStore) Insert(c *model.Calibration) error {
	_, err := s.db.SQL().Exec(
		`INSERT INTO calibrations (id, batch_id, status, bias_deg, sample_size, confidence, created_at, activated_at, revoked_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.BatchID, string(c.Status), c.BiasDeg, c.SampleSize, c.Confidence,
		ts(c.CreatedAt), tsNull(c.ActivatedAt), tsNull(c.RevokedAt),
	)
	if err != nil {
		return fmt.Errorf("insert calibration: %w", err)
	}
	return nil
}

// Get 按 ID 取校准版本。
func (s *CalibrationStore) Get(id string) (*model.Calibration, error) {
	row := s.db.SQL().QueryRow(
		`SELECT id, batch_id, status, bias_deg, sample_size, confidence, created_at, activated_at, revoked_at
		 FROM calibrations WHERE id = ?`, id)
	c, err := scanCalibration(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

// ListByBatch 列出批次下校准版本（按创建时间倒序）。
func (s *CalibrationStore) ListByBatch(batchID string) ([]*model.Calibration, error) {
	rows, err := s.db.SQL().Query(
		`SELECT id, batch_id, status, bias_deg, sample_size, confidence, created_at, activated_at, revoked_at
		 FROM calibrations WHERE batch_id = ? ORDER BY created_at DESC`, batchID)
	if err != nil {
		return nil, fmt.Errorf("list calibrations: %w", err)
	}
	defer rows.Close()
	var out []*model.Calibration
	for rows.Next() {
		c, err := scanCalibration(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Active 返回批次当前生效的校准版本（至多一个）。
func (s *CalibrationStore) Active(batchID string) (*model.Calibration, error) {
	row := s.db.SQL().QueryRow(
		`SELECT id, batch_id, status, bias_deg, sample_size, confidence, created_at, activated_at, revoked_at
		 FROM calibrations WHERE batch_id = ? AND status = ? ORDER BY created_at DESC LIMIT 1`,
		batchID, string(model.CalibrationActive))
	c, err := scanCalibration(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

// Update 更新校准版本。
func (s *CalibrationStore) Update(c *model.Calibration) error {
	_, err := s.db.SQL().Exec(
		`UPDATE calibrations SET status=?, bias_deg=?, sample_size=?, confidence=?, activated_at=?, revoked_at=?
		 WHERE id=?`,
		string(c.Status), c.BiasDeg, c.SampleSize, c.Confidence, tsNull(c.ActivatedAt), tsNull(c.RevokedAt), c.ID,
	)
	if err != nil {
		return fmt.Errorf("update calibration: %w", err)
	}
	return nil
}

// RevokeAllActive 把批次全部生效版本废止（激活新版本前调用，事务内）。
func (s *CalibrationStore) RevokeAllActive(batchID string) error {
	_, err := s.db.SQL().Exec(
		`UPDATE calibrations SET status=?, revoked_at=? WHERE batch_id=? AND status=?`,
		string(model.CalibrationRevoked), ts(nowUTC()), batchID, string(model.CalibrationDraft),
	)
	if err != nil {
		return fmt.Errorf("revoke all active: %w", err)
	}
	return nil
}

func scanCalibration(sc scanner) (*model.Calibration, error) {
	var (
		c            model.Calibration
		status       string
		createdRaw   string
		activatedRaw sql.NullString
		revokedRaw   sql.NullString
	)
	if err := sc.Scan(&c.ID, &c.BatchID, &status, &c.BiasDeg, &c.SampleSize, &c.Confidence,
		&createdRaw, &activatedRaw, &revokedRaw); err != nil {
		return nil, err
	}
	c.Status = model.CalibrationStatus(status)
	var err error
	c.CreatedAt, err = parseTS(createdRaw)
	if err != nil {
		return nil, err
	}
	if activatedRaw.Valid {
		t, err := parseTS(activatedRaw.String)
		if err != nil {
			return nil, err
		}
		c.ActivatedAt = &t
	}
	if revokedRaw.Valid {
		t, err := parseTS(revokedRaw.String)
		if err != nil {
			return nil, err
		}
		c.RevokedAt = &t
	}
	return &c, nil
}
