package store

import (
	"database/sql"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"task200-fiberorient/internal/model"
)

// seedBatch 插入批次行（results 表有 batches 外键依赖）。
func seedBatch(db *DB, id string) error {
	_, err := db.SQL().Exec(
		`INSERT INTO batches (id, name, material, slice_angle, status, version, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, "t", "paper", 0, string(model.BatchAnalyzable), 1, "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z")
	return err
}

// TestResultStoreAllocateAndInsertConcurrent 验证同一批次并发提交多次结果时，
// AllocateAndInsert 分配的版本号唯一且连续（1..N，无重复、无唯一键冲突错误）。
// 这是 bug7 的核心回归测试：旧实现 NextVersion+Insert 两步分配在 20 并发下会
// 产生重复版本与 UNIQUE(batch_id, version) 冲突。
func TestResultStoreAllocateAndInsertConcurrent(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	const batchID = "batch-conc"
	if err := seedBatch(db, batchID); err != nil {
		t.Fatalf("seed batch: %v", err)
	}
	s := NewResultStore(db)

	const n = 20
	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		versions  = map[int]int{}
		conflicts int
		otherErrs []error
	)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// 以空 ID 构造：ID 由 AllocateAndInsert 在事务内确定版本后回写。
			r, err := model.NewResult("", batchID, 1, "cal-x", "f", 1)
			if err != nil {
				mu.Lock()
				otherErrs = append(otherErrs, err)
				mu.Unlock()
				return
			}
			if err := s.AllocateAndInsert(r); err != nil {
				mu.Lock()
				if errors.Is(err, model.ErrConflict) {
					conflicts++
				} else {
					otherErrs = append(otherErrs, err)
				}
				mu.Unlock()
				return
			}
			mu.Lock()
			versions[r.Version]++
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	if len(otherErrs) != 0 {
		t.Fatalf("unexpected errors: %v", otherErrs)
	}
	if conflicts != 0 {
		t.Fatalf("got %d unique-key conflicts, want 0", conflicts)
	}
	if len(versions) != n {
		t.Fatalf("got %d distinct versions, want %d (duplicates present)", len(versions), n)
	}
	for v := 1; v <= n; v++ {
		if versions[v] != 1 {
			t.Fatalf("version %d allocated %d times, want exactly 1", v, versions[v])
		}
	}

	// 落库行数与版本一致：恰好 N 条，版本号 1..N 连续无缺口。
	rows, err := db.SQL().Query(
		`SELECT version FROM results WHERE batch_id = ? ORDER BY version`, batchID)
	if err != nil {
		t.Fatalf("query results: %v", err)
	}
	defer rows.Close()
	got := []int{}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("scan version: %v", err)
		}
		got = append(got, v)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows err: %v", err)
	}
	if len(got) != n {
		t.Fatalf("persisted %d rows, want %d", len(got), n)
	}
	for i, v := range got {
		if v != i+1 {
			t.Fatalf("version at index %d = %d, want %d (not contiguous)", i, v, i+1)
		}
	}
}

// TestResultStoreTwoStepVersionRaceRepro 复现旧“读 MAX(version)+1 再 Insert”两步
// 分配的并发缺陷，作为对照：在 BEGIN IMMEDIATE 事务之外读取并插入，20 并发下应
// 出现重复版本或唯一键冲突。修复后业务不再走此路径，本测试锁定缺陷形态以防回退。
func TestResultStoreTwoStepVersionRaceRepro(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	const batchID = "batch-race"
	if err := seedBatch(db, batchID); err != nil {
		t.Fatalf("seed batch: %v", err)
	}
	s := NewResultStore(db)

	const n = 20
	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		versions  = map[int]int{}
		conflicts int
		otherErrs int
		oks       int
	)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// 旧两步实现：事务外读 MAX(version)，再裸 Insert（不持写锁）。
			var maxV sql.NullInt64
			if err := db.SQL().QueryRow(
				`SELECT MAX(version) FROM results WHERE batch_id = ?`, batchID).Scan(&maxV); err != nil {
				mu.Lock()
				otherErrs++
				mu.Unlock()
				return
			}
			v := 1
			if maxV.Valid {
				v = int(maxV.Int64) + 1
			}
			// 放大竞态窗口（模拟旧实现的 runtime.Gosched / time.Sleep）。
			runtime.Gosched()
			time.Sleep(time.Millisecond)
			r, err := model.NewResult(
				fmt.Sprintf("res-%s-%d", batchID, i), batchID, v, "cal-x", "f", 1)
			if err != nil {
				mu.Lock()
				otherErrs++
				mu.Unlock()
				return
			}
			if err := s.Insert(r); err != nil {
				mu.Lock()
				if isUniqueViolation(err) {
					conflicts++
				} else {
					otherErrs++
				}
				mu.Unlock()
				return
			}
			mu.Lock()
			versions[v]++
			oks++
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	// 旧两步实现必然失败：重复版本（len(versions) < oks）或唯一键冲突 > 0。
	if conflicts == 0 && len(versions) == oks {
		t.Fatalf("two-step race did not surface the bug: oks=%d distinctVersions=%d conflicts=%d",
			oks, len(versions), conflicts)
	}
	t.Logf("two-step race: oks=%d distinctVersions=%d conflicts=%d otherErrs=%d",
		oks, len(versions), conflicts, otherErrs)
}
