// Package store 提供基于 SQLite（modernc.org/sqlite，纯 Go 驱动，CGO 无关）的
// 持久化实现：建表迁移、批/视野/观测/校准/结果的 CRUD 与查询。
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Open 打开（或创建）SQLite 数据库并执行迁移。
// 支持 ":memory:" 用于测试与冒烟验证。
func Open(path string) (*DB, error) {
	if path == ":memory:" {
		db, err := sql.Open("sqlite", "file::memory:?cache=shared")
		if err != nil {
			return nil, fmt.Errorf("open memory db: %w", err)
		}
		db.SetMaxOpenConns(8)
		d := &DB{db: db, path: path}
		if err := d.migrate(); err != nil {
			db.Close()
			return nil, err
		}
		return d, nil
	}

	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("mkdir db dir: %w", err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	db.SetMaxOpenConns(8)
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set wal: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable fk: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}
	d := &DB{db: db, path: path}
	if err := d.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return d, nil
}

// DB 封装 SQLite 连接。
type DB struct {
	db   *sql.DB
	path string
}

// Close 关闭数据库连接。
func (d *DB) Close() error { return d.db.Close() }

// Path 返回数据库路径（调试用）。
func (d *DB) Path() string { return d.path }

// SQL 暴露底层连接（供 Store 实现使用）。
func (d *DB) SQL() *sql.DB { return d.db }

// migrate 建表：全部业务表 + 唯一约束（防重复）。
func (d *DB) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS batches (
			id           TEXT PRIMARY KEY,
			name         TEXT NOT NULL,
			material     TEXT NOT NULL,
			slice_angle  REAL NOT NULL,
			status       TEXT NOT NULL,
			version      INTEGER NOT NULL,
			created_at   TEXT NOT NULL,
			updated_at   TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS fields (
			id         TEXT PRIMARY KEY,
			batch_id   TEXT NOT NULL REFERENCES batches(id),
			label      TEXT NOT NULL,
			slice_deg  REAL NOT NULL,
			status     TEXT NOT NULL,
			pollute_by TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_fields_batch ON fields(batch_id)`,
		`CREATE TABLE IF NOT EXISTS observations (
			id          TEXT PRIMARY KEY,
			batch_id    TEXT NOT NULL REFERENCES batches(id),
			field_id    TEXT NOT NULL REFERENCES fields(id),
			angle_deg   REAL NOT NULL,
			unit        TEXT NOT NULL,
			fingerprint TEXT NOT NULL UNIQUE,
			created_at  TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_obs_batch ON observations(batch_id)`,
		`CREATE INDEX IF NOT EXISTS idx_obs_field ON observations(field_id)`,
		`CREATE TABLE IF NOT EXISTS calibrations (
			id          TEXT PRIMARY KEY,
			batch_id    TEXT NOT NULL REFERENCES batches(id),
			status      TEXT NOT NULL,
			bias_deg    REAL NOT NULL,
			sample_size INTEGER NOT NULL,
			confidence  REAL NOT NULL,
			created_at  TEXT NOT NULL,
			activated_at TEXT,
			revoked_at  TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_cal_batch ON calibrations(batch_id)`,
		`CREATE TABLE IF NOT EXISTS results (
			id                 TEXT PRIMARY KEY,
			batch_id           TEXT NOT NULL REFERENCES batches(id),
			version            INTEGER NOT NULL,
			status             TEXT NOT NULL,
			calibration_id     TEXT NOT NULL,
			field_snapshot     TEXT NOT NULL,
			observation_count  INTEGER NOT NULL,
			mean_deg           REAL,
			resultant_length   REAL,
			circular_variance  REAL,
			kappa              REAL,
			rayleigh_p         REAL,
			is_bimodal         INTEGER,
			primary_deg        REAL,
			secondary_deg      REAL,
			dip_statistic      REAL,
			separation_deg     REAL,
			ci_lower_deg       REAL,
			ci_upper_deg       REAL,
			ci_level           REAL,
			ci_method          TEXT,
			created_at         TEXT NOT NULL,
			frozen_at          TEXT,
			UNIQUE(batch_id, version)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_results_batch ON results(batch_id)`,
	}
	for _, s := range stmts {
		if _, err := d.db.Exec(s); err != nil {
			return fmt.Errorf("migrate: %w (%s)", err, firstWords(s, 12))
		}
	}
	return nil
}

func firstWords(s string, n int) string {
	parts := strings.Fields(s)
	if len(parts) > n {
		parts = parts[:n]
	}
	return strings.Join(parts, " ")
}

func nowUTC() time.Time { return time.Now().UTC() }
