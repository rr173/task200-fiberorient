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

// NextSeq 计算批次下一个校准序号（现有版本数+1），用于生成稳定且唯一的草稿 ID。
func (s *CalibrationStore) NextSeq(batchID string) (int, error) {
	var count int64
	if err := s.db.SQL().QueryRow(
		`SELECT COUNT(*) FROM calibrations WHERE batch_id = ?`, batchID).Scan(&count); err != nil {
		return 0, fmt.Errorf("next calibration seq: %w", err)
	}
	return int(count) + 1, nil
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

// CountActive 返回批次下生效版本数量（不变更数据，供一致性自检）。
func (s *CalibrationStore) CountActive(batchID string) (int, error) {
	var count int64
	if err := s.db.SQL().QueryRow(
		`SELECT COUNT(*) FROM calibrations WHERE batch_id = ? AND status = ?`,
		batchID, string(model.CalibrationActive)).Scan(&count); err != nil {
		return 0, fmt.Errorf("count active calibrations: %w", err)
	}
	return int(count), nil
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

// RevokeAllActive 把批次全部生效版本废止（激活新版本前调用）。
// 必须按 batch_id 限定范围并匹配 status=active，否则会误废止其他批次的生效版本。
func (s *CalibrationStore) RevokeAllActive(batchID string) error {
	if batchID == "" {
		return fmt.Errorf("revoke all active: batch id required")
	}
	res, err := s.db.SQL().Exec(
		`UPDATE calibrations SET status=?, revoked_at=? WHERE batch_id=? AND status=?`,
		string(model.CalibrationRevoked), ts(nowUTC()), batchID, string(model.CalibrationActive))
	if err != nil {
		return fmt.Errorf("revoke all active: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("revoke all active: rows affected: %w", err)
	}
	// 同一批次至多一个生效版本；超过即说明并发或历史数据不一致，激活应中止以免留下歧义状态。
	if n > 1 {
		return fmt.Errorf("revoke all active: batch %s has %d active calibrations, expected at most 1", batchID, n)
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
