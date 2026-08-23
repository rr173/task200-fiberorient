package store

import (
	"database/sql"
	"errors"
	"fmt"

	"task200-fiberorient/internal/model"
)

// ResultStore 统计结果版本持久化。
type ResultStore struct{ db *DB }

// NewResultStore 构造结果仓储。
func NewResultStore(db *DB) *ResultStore { return &ResultStore{db: db} }

// Insert 插入结果版本（UNIQUE(batch_id, version) 防并发重复版本）。
func (s *ResultStore) Insert(r *model.Result) error {
	_, err := s.db.SQL().Exec(
		`INSERT INTO results (
			id, batch_id, version, status, calibration_id, field_snapshot, observation_count,
			mean_deg, resultant_length, circular_variance, kappa, rayleigh_p,
			is_bimodal, primary_deg, secondary_deg, dip_statistic, separation_deg,
			ci_lower_deg, ci_upper_deg, ci_level, ci_method, created_at, frozen_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.BatchID, r.Version, string(r.Status), r.CalibrationID, r.FieldSnapshot, r.ObservationCount,
		nullableFloat(r.Stats, "mean"), nullableFloat(r.Stats, "rlen"), nullableFloat(r.Stats, "var"),
		nullableFloat(r.Stats, "kappa"), nullableFloat(r.Stats, "rayleigh"),
		nullableBool(r.Bimodality), nullableFloatB(r.Bimodality, "primary"), nullableFloatB(r.Bimodality, "secondary"),
		nullableFloatB(r.Bimodality, "dip"), nullableFloatB(r.Bimodality, "separation"),
		nullableFloatCI(r.Confidence, "lower"), nullableFloatCI(r.Confidence, "upper"),
		nullableFloatCI(r.Confidence, "level"), nullableStringCI(r.Confidence),
		ts(r.CreatedAt), tsNull(r.FrozenAt),
	)
	if err != nil {
		return fmt.Errorf("insert result: %w", err)
	}
	return nil
}

// Get 按 ID 取结果。
func (s *ResultStore) Get(id string) (*model.Result, error) {
	row := s.db.SQL().QueryRow(resultColumns+" FROM results WHERE id = ?", id)
	r, err := scanResult(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return r, nil
}

// ListByBatch 列出批次结果版本（版本号倒序）。
func (s *ResultStore) ListByBatch(batchID string) ([]*model.Result, error) {
	rows, err := s.db.SQL().Query(
		resultColumns+" FROM results WHERE batch_id = ? ORDER BY version ASC", batchID)
	if err != nil {
		return nil, fmt.Errorf("list results: %w", err)
	}
	defer rows.Close()
	var out []*model.Result
	for rows.Next() {
		r, err := scanResult(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// NextVersion 计算批次下一个结果版本号（事务内串行保证唯一）。
func (s *ResultStore) NextVersion(batchID string) (int, error) {
	var maxV sql.NullInt64
	if err := s.db.SQL().QueryRow(
		`SELECT MAX(version) FROM results WHERE batch_id = ?`, batchID).Scan(&maxV); err != nil {
		return 0, fmt.Errorf("next version: %w", err)
	}
	if !maxV.Valid {
		return 1, nil
	}
	return int(maxV.Int64) + 1, nil
}

// Update 更新结果（含冻结时间戳）。
func (s *ResultStore) Update(r *model.Result) error {
	_, err := s.db.SQL().Exec(
		`UPDATE results SET status=?, mean_deg=?, resultant_length=?, circular_variance=?, kappa=?, rayleigh_p=?,
			is_bimodal=?, primary_deg=?, secondary_deg=?, dip_statistic=?, separation_deg=?,
			ci_lower_deg=?, ci_upper_deg=?, ci_level=?, ci_method=?, frozen_at=?
		 WHERE id=?`,
		string(r.Status),
		nullableFloat(r.Stats, "mean"), nullableFloat(r.Stats, "rlen"), nullableFloat(r.Stats, "var"),
		nullableFloat(r.Stats, "kappa"), nullableFloat(r.Stats, "rayleigh"),
		nullableBool(r.Bimodality), nullableFloatB(r.Bimodality, "primary"), nullableFloatB(r.Bimodality, "secondary"),
		nullableFloatB(r.Bimodality, "dip"), nullableFloatB(r.Bimodality, "separation"),
		nullableFloatCI(r.Confidence, "lower"), nullableFloatCI(r.Confidence, "upper"),
		nullableFloatCI(r.Confidence, "level"), nullableStringCI(r.Confidence),
		tsNull(r.FrozenAt), r.ID,
	)
	if err != nil {
		return fmt.Errorf("update result: %w", err)
	}
	return nil
}

const resultColumns = `SELECT id, batch_id, version, status, calibration_id, field_snapshot, observation_count,
	mean_deg, resultant_length, circular_variance, kappa, rayleigh_p,
	is_bimodal, primary_deg, secondary_deg, dip_statistic, separation_deg,
	ci_lower_deg, ci_upper_deg, ci_level, ci_method, created_at, frozen_at`

func scanResult(sc scanner) (*model.Result, error) {
	var (
		r          model.Result
		status     string
		mean, rlen sql.NullFloat64
		variance   sql.NullFloat64
		kappa      sql.NullFloat64
		rayleigh   sql.NullFloat64
		bimodal    sql.NullInt64
		primary    sql.NullFloat64
		secondary  sql.NullFloat64
		dip        sql.NullFloat64
		separation sql.NullFloat64
		ciLower    sql.NullFloat64
		ciUpper    sql.NullFloat64
		ciLevel    sql.NullFloat64
		ciMethod   sql.NullString
		createdRaw string
		frozenRaw  sql.NullString
	)
	if err := sc.Scan(&r.ID, &r.BatchID, &r.Version, &status, &r.CalibrationID, &r.FieldSnapshot,
		&r.ObservationCount, &mean, &rlen, &variance, &kappa, &rayleigh,
		&bimodal, &primary, &secondary, &dip, &separation,
		&ciLower, &ciUpper, &ciLevel, &ciMethod, &createdRaw, &frozenRaw); err != nil {
		return nil, err
	}
	r.Status = model.ResultStatus(status)
	if mean.Valid {
		r.Stats = &model.CircularStats{
			MeanDeg:          mean.Float64,
			ResultantLength:  rlen.Float64,
			CircularVariance: variance.Float64,
			Kappa:            kappa.Float64,
			RayleighP:        rayleigh.Float64,
		}
	}
	if bimodal.Valid {
		r.Bimodality = &model.Bimodality{
			IsBimodal:     bimodal.Int64 == 1,
			PrimaryDeg:    primary.Float64,
			SecondaryDeg:  secondary.Float64,
			DipStatistic:  dip.Float64,
			SeparationDeg: separation.Float64,
		}
	}
	if ciLower.Valid {
		r.Confidence = &model.ConfidenceInterval{
			LowerDeg: ciLower.Float64,
			UpperDeg: ciUpper.Float64,
			Level:    ciLevel.Float64,
			Method:   ciMethod.String,
		}
	}
	var err error
	r.CreatedAt, err = parseTS(createdRaw)
	if err != nil {
		return nil, err
	}
	if frozenRaw.Valid {
		t, err := parseTS(frozenRaw.String)
		if err != nil {
			return nil, err
		}
		r.FrozenAt = &t
	}
	return &r, nil
}

func nullableFloat(s *model.CircularStats, key string) any {
	if s == nil {
		return nil
	}
	switch key {
	case "mean":
		return s.MeanDeg
	case "rlen":
		return s.ResultantLength
	case "var":
		return s.CircularVariance
	case "kappa":
		return s.Kappa
	case "rayleigh":
		return s.RayleighP
	}
	return nil
}

func nullableFloatB(b *model.Bimodality, key string) any {
	if b == nil {
		return nil
	}
	switch key {
	case "primary":
		return b.PrimaryDeg
	case "secondary":
		return b.SecondaryDeg
	case "dip":
		return b.DipStatistic
	case "separation":
		return b.SeparationDeg
	}
	return nil
}

func nullableBool(b *model.Bimodality) any {
	if b == nil {
		return nil
	}
	if b.IsBimodal {
		return 1
	}
	return 0
}

func nullableFloatCI(ci *model.ConfidenceInterval, key string) any {
	if ci == nil {
		return nil
	}
	switch key {
	case "lower":
		return ci.LowerDeg
	case "upper":
		return ci.UpperDeg
	case "level":
		return ci.Level
	}
	return nil
}

func nullableStringCI(ci *model.ConfidenceInterval) any {
	if ci == nil {
		return nil
	}
	return ci.Method
}
