package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"task200-fiberorient/internal/model"
)

// ResultStore 统计结果版本持久化。
type ResultStore struct{ db *DB }

// NewResultStore 构造结果仓储。
func NewResultStore(db *DB) *ResultStore { return &ResultStore{db: db} }

// resultInsertSQL 结果插入语句；列序由 scanResult 镜像，供 AllocateAndInsert 与
// Insert 共用，避免两处列序漂移。
const resultInsertSQL = `INSERT INTO results (
	id, batch_id, version, status, calibration_id, field_snapshot, observation_count,
	mean_deg, resultant_length, circular_variance, kappa, rayleigh_p,
	is_bimodal, primary_deg, secondary_deg, dip_statistic, separation_deg,
	ci_lower_deg, ci_upper_deg, ci_level, ci_method, created_at, frozen_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

func resultInsertArgs(r *model.Result) []any {
	return []any{
		r.ID, r.BatchID, r.Version, string(r.Status), r.CalibrationID, r.FieldSnapshot, r.ObservationCount,
		nullableFloat(r.Stats, "mean"), nullableFloat(r.Stats, "rlen"), nullableFloat(r.Stats, "var"),
		nullableFloat(r.Stats, "kappa"), nullableFloat(r.Stats, "rayleigh"),
		nullableBool(r.Bimodality), nullableFloatB(r.Bimodality, "primary"), nullableFloatB(r.Bimodality, "secondary"),
		nullableFloatB(r.Bimodality, "dip"), nullableFloatB(r.Bimodality, "separation"),
		nullableFloatCI(r.Confidence, "lower"), nullableFloatCI(r.Confidence, "upper"),
		nullableFloatCI(r.Confidence, "level"), nullableStringCI(r.Confidence),
		ts(r.CreatedAt), tsNull(r.FrozenAt),
	}
}

// AllocateAndInsert 原子地分配版本号并落库结果版本。
//
// 把“取下一版本号”与“插入结果行”收敛进同一条 BEGIN IMMEDIATE 事务（DSN 的
// _txlock=immediate 使 db.BeginTx 即发出 BEGIN IMMEDIATE，在读取 MAX(version) 前
// 即获取写锁）：保证同一批次并发计算时版本号串行递增、唯一且连续，杜绝旧两步
// 实现（NextVersion + Insert）下的重复版本与 UNIQUE(batch_id, version) 冲突。
// SQLite 同一时刻仅允许一个写事务，并发写入方在 busy_timeout 内自动排队；共享
// 内存库下 busy 行为不总是可靠，故对 SQLITE_BUSY/locked 再做一轮指数退避重试。
// 若仍撞上唯一约束（理论不应发生，留作最后防线），映射为 ErrConflict 由上层处理。
func (s *ResultStore) AllocateAndInsert(r *model.Result) error {
	const maxRetry = 16
	backoff := 2 * time.Millisecond
	var lastErr error
	for attempt := 0; attempt <= maxRetry; attempt++ {
		err := s.allocAndInsertOnce(r)
		if err == nil {
			return nil
		}
		lastErr = err
		switch {
		case isUniqueViolation(err):
			// 版本号冲突：另一事务已先行提交相同版本。重试会在下一轮读到更高
			// 的 MAX(version)，从而递增到不冲突的版本号；退避可降低再次相撞概率。
			// 重试耗尽则上抛冲突语义。
			if attempt == maxRetry {
				return model.ErrConflict
			}
		case isBusyOrLocked(err):
			// 写锁争用：退避后重试。
			if attempt == maxRetry {
				return fmt.Errorf("allocate result: %w", lastErr)
			}
		default:
			return err
		}
		time.Sleep(backoff)
		backoff *= 2
		if backoff > 50*time.Millisecond {
			backoff = 50 * time.Millisecond
		}
	}
	return fmt.Errorf("allocate result: %w", lastErr)
}

// allocAndInsertOnce 执行一次“开写事务 → 取版本 → 回写 ID → 插入 → 提交”的完整尝试。
// db 已配置 _txlock=immediate，Begin() 发出 BEGIN IMMEDIATE，写锁在 SELECT 前
// 即持有。版本号确定后回写 r.ID 为 res-<batch>-v<version>，与版本号保持一致，
// 便于人读与日志追踪。
func (s *ResultStore) allocAndInsertOnce(r *model.Result) error {
	tx, err := s.db.SQL().Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }() // 已 Commit 后的回滚为空操作

	var maxV sql.NullInt64
	if err := tx.QueryRow(
		`SELECT MAX(version) FROM results WHERE batch_id = ?`, r.BatchID).Scan(&maxV); err != nil {
		return fmt.Errorf("next version: %w", err)
	}
	if maxV.Valid {
		r.Version = int(maxV.Int64) + 1
	} else {
		r.Version = 1
	}
	// 版本号在事务内确定后回写 ID，保证 ID 与版本一致且落库即最终态。
	r.ID = fmt.Sprintf("res-%s-v%d", r.BatchID, r.Version)

	if _, err := tx.Exec(resultInsertSQL, resultInsertArgs(r)...); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

// isUniqueViolation 判断是否为唯一约束冲突。
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint failed") ||
		strings.Contains(msg, "constraint failed: unique") ||
		strings.Contains(msg, "sqlite_constraint_unique")
}

// isBusyOrLocked 判断是否为写锁争用（可退避重试）。
func isBusyOrLocked(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "database is locked") ||
		strings.Contains(msg, "database table is locked") ||
		strings.Contains(msg, "sqlite_error_busy") ||
		strings.Contains(msg, "would block")
}

// Insert 插入结果版本（UNIQUE(batch_id, version) 防并发重复版本）。
// 注意：调用方需自行保证 r.Version 已正确分配；并发场景应优先使用
// AllocateAndInsert（版本分配与落库原子化）。保留 Insert 供已确定版本号的
// 非并发路径使用。
func (s *ResultStore) Insert(r *model.Result) error {
	_, err := s.db.SQL().Exec(resultInsertSQL, resultInsertArgs(r)...)
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
		resultColumns+" FROM results WHERE batch_id = ? ORDER BY version DESC", batchID)
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
