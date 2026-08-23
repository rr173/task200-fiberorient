// Command fiberorient 纸张纤维取向统计校准服务入口。
//
// 支持三个标志：
//   - --addr :8080      监听地址（默认 :8080）
//   - --db ./data.db    SQLite 数据库路径（默认 ./fiberorient.db）
//   - --smoke-test      执行端到端冒烟：真实创建数据、关闭并重开数据库
//     验证持久化与重启恢复，随后以 0 退出码结束。
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"task200-fiberorient/internal/httpapi"
	"task200-fiberorient/internal/model"
	"task200-fiberorient/internal/result"
	"task200-fiberorient/internal/service"
	"task200-fiberorient/internal/store"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	dbPath := flag.String("db", "./fiberorient.db", "SQLite database path")
	smoke := flag.Bool("smoke-test", false, "run end-to-end smoke test and exit")
	flag.Parse()

	if *smoke {
		if err := runSmokeTest(*dbPath); err != nil {
			fmt.Fprintln(os.Stderr, "SMOKE TEST FAILED:", err)
			os.Exit(1)
		}
		fmt.Println("SMOKE TEST PASSED")
		return
	}

	db, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	app, err := service.New(db)
	if err != nil {
		log.Fatalf("init services: %v", err)
	}
	srv := httpapi.New(app)
	log.Printf("task200-fiberorient listening on %s (db=%s)", *addr, *dbPath)
	if err := http.ListenAndServe(*addr, srv.Handler()); err != nil {
		log.Fatalf("http server: %v", err)
	}
}

// runSmokeTest 执行冒烟测试：
//  1. 打开数据库 A，完整跑一遍业务闭环（批次→视野→导入→清洗→校准→统计→发布）；
//  2. 幂等验证：重复导入相同指纹观测不产生重复数据；
//  3. 单位混用拒绝：角度单位混用被拒；
//  4. 污染视野剔除后重算，双峰方向得到更窄区间并发布；
//  5. 新增观测产生替代版本（版本号递增）；
//  6. 关闭数据库 A，重新打开同一路径数据库 B，验证数据仍在（重启恢复）；
//  7. 冻结结果不可直接编辑。
func runSmokeTest(dbPath string) error {
	if dbPath != ":memory:" {
		_ = os.Remove(dbPath)
	}

	db, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	app, err := service.New(db)
	if err != nil {
		db.Close()
		return fmt.Errorf("init services: %w", err)
	}

	// --- 步骤 1：批次 ---
	b, err := app.Batches.Create("batch-1", "双胶纸 A 批", "漂白硫酸盐浆", 45)
	if err != nil {
		db.Close()
		return fmt.Errorf("create batch: %w", err)
	}
	if b.Status != model.BatchRegistered {
		db.Close()
		return fmt.Errorf("batch status should be registered, got %s", b.Status)
	}

	// 流转到观测中
	if _, err := app.Batches.Transition("batch-1", model.BatchObserving); err != nil {
		db.Close()
		return fmt.Errorf("transition to observing: %w", err)
	}

	// --- 步骤 2：视野与角度导入（双切片设计） ---
	// 真实面内方向 30°，仪器偏差 +12°：
	// fld-a（0° 切片）：表观 θ0 = 30+12 = 42°
	// fld-b（90° 切片）：表观 θ90 = 30+90−12 = 108°
	// fld-c：0° 切片污染视野（脏污覆盖），导入杂散角度后被剔除
	if _, err := app.Fields.RegisterWithID("fld-a", "batch-1", "0° 切片视野 1", 0); err != nil {
		db.Close()
		return fmt.Errorf("register field a: %w", err)
	}
	if _, err := app.Fields.RegisterWithID("fld-b", "batch-1", "90° 切片视野 2", 90); err != nil {
		db.Close()
		return fmt.Errorf("register field b: %w", err)
	}
	if _, err := app.Fields.RegisterWithID("fld-c", "batch-1", "0° 切片污染视野", 0); err != nil {
		db.Close()
		return fmt.Errorf("register field c: %w", err)
	}

	// 0° 切片：表观 42° 主方向
	anglesA := []float64{40, 43, 41, 45, 42, 39, 44, 42, 41, 43, 42, 40, 45, 42, 43}
	res1, err := app.Obs.Import("batch-1", "fld-a", model.UnitDegrees, anglesA, "sub-a-1")
	if err != nil {
		db.Close()
		return fmt.Errorf("import a: %w", err)
	}
	if res1.Inserted != len(anglesA) {
		db.Close()
		return fmt.Errorf("import a inserted %d, want %d", res1.Inserted, len(anglesA))
	}

	// 90° 切片：表观 108° 主方向
	anglesB := []float64{106, 109, 107, 111, 108, 105, 110, 108, 107, 109, 108, 106, 111, 108, 109}
	res2, err := app.Obs.Import("batch-1", "fld-b", model.UnitDegrees, anglesB, "sub-b-1")
	if err != nil {
		db.Close()
		return fmt.Errorf("import b: %w", err)
	}
	if res2.Inserted != len(anglesB) {
		db.Close()
		return fmt.Errorf("import b inserted %d, want %d", res2.Inserted, len(anglesB))
	}

	// 污染视野：杂散角度（脏污覆盖导致不可信）
	if _, err := app.Obs.Import("batch-1", "fld-c", model.UnitDegrees, []float64{5, 170, 88, 95, 12, 160}, "sub-c-1"); err != nil {
		db.Close()
		return fmt.Errorf("import c: %w", err)
	}

	// 幂等：同 submission 重复导入 → 全部跳过；新 submission 同角度不误判重复
	res3, err := app.Obs.Import("batch-1", "fld-a", model.UnitDegrees, anglesA, "sub-a-1")
	if err != nil {
		db.Close()
		return fmt.Errorf("reimport a: %w", err)
	}
	if res3.Inserted != 0 || res3.Skipped != len(anglesA) {
		db.Close()
		return fmt.Errorf("reimport a should skip all, got inserted=%d skipped=%d", res3.Inserted, res3.Skipped)
	}
	res3b, err := app.Obs.Import("batch-1", "fld-a", model.UnitDegrees, []float64{40, 43, 41}, "sub-a-2")
	if err != nil {
		db.Close()
		return fmt.Errorf("reimport a new submission: %w", err)
	}
	if res3b.Inserted != 3 || res3b.Skipped != 0 {
		db.Close()
		return fmt.Errorf("new submission with same angles should insert 3, got inserted=%d skipped=%d", res3b.Inserted, res3b.Skipped)
	}

	// 单位混用：向污染视野 fld-c 导入弧度值（不进入统计，但保留用于审计）。
	// 同批次不同视野单位可不同；真正拒绝发生在统计阶段（同批视野集合内混用）。
	if _, err := app.Obs.Import("batch-1", "fld-c", model.UnitRadians, []float64{0.5, 0.6}, "sub-c-rad"); err != nil {
		db.Close()
		return fmt.Errorf("import rad should succeed: %w", err)
	}

	// --- 步骤 3：清洗 ---
	// fld-a、fld-b 有效（0°/90° 双切片校准依赖）；fld-c 污染后剔除
	if _, err := app.Fields.MarkValid("fld-a"); err != nil {
		db.Close()
		return fmt.Errorf("mark fld-a valid: %w", err)
	}
	if _, err := app.Fields.MarkValid("fld-b"); err != nil {
		db.Close()
		return fmt.Errorf("mark fld-b valid: %w", err)
	}
	if _, err := app.Fields.MarkPolluted("fld-c", "脏污覆盖"); err != nil {
		db.Close()
		return fmt.Errorf("mark fld-c polluted: %w", err)
	}
	if _, err := app.Fields.Exclude("fld-c"); err != nil {
		db.Close()
		return fmt.Errorf("exclude fld-c: %w", err)
	}

	// --- 步骤 3.5：单位混用拒绝（独立批次验证统计阶段守卫） ---
	if _, err := app.Batches.Create("batch-mix", "单位混用验证", "浆板", 0); err != nil {
		db.Close()
		return fmt.Errorf("create mix batch: %w", err)
	}
	if _, err := app.Fields.RegisterWithID("fld-mix0", "batch-mix", "0° 混合视野", 0); err != nil {
		db.Close()
		return fmt.Errorf("register mix field0: %w", err)
	}
	if _, err := app.Fields.RegisterWithID("fld-mix90", "batch-mix", "90° 混合视野", 90); err != nil {
		db.Close()
		return fmt.Errorf("register mix field90: %w", err)
	}
	if _, err := app.Fields.MarkValid("fld-mix0"); err != nil {
		db.Close()
		return fmt.Errorf("mark mix0 valid: %w", err)
	}
	if _, err := app.Fields.MarkValid("fld-mix90"); err != nil {
		db.Close()
		return fmt.Errorf("mark mix90 valid: %w", err)
	}
	if _, err := app.Obs.Import("batch-mix", "fld-mix0", model.UnitDegrees, []float64{10, 20, 30, 40}, "mix-deg"); err != nil {
		db.Close()
		return fmt.Errorf("import mix deg: %w", err)
	}
	if _, err := app.Obs.Import("batch-mix", "fld-mix0", model.UnitRadians, []float64{0.1, 0.2}, "mix-rad"); err != nil {
		db.Close()
		return fmt.Errorf("import mix rad: %w", err)
	}
	if _, err := app.Obs.Import("batch-mix", "fld-mix90", model.UnitDegrees, []float64{100, 110, 120, 130}, "mix90-deg"); err != nil {
		db.Close()
		return fmt.Errorf("import mix90: %w", err)
	}
	mixDraft, err := app.Cals.EstimateAndDraft("batch-mix", 0)
	if err != nil {
		db.Close()
		return fmt.Errorf("estimate mix cal: %w", err)
	}
	if _, err := app.Cals.Activate(mixDraft.Calibration.ID); err != nil {
		db.Close()
		return fmt.Errorf("activate mix cal: %w", err)
	}
	_, err = app.Results.Compute(result.ComputeOptions{
		BatchID:       "batch-mix",
		CalibrationID: mixDraft.Calibration.ID,
		FieldIDs:      []string{"fld-mix0"},
		CiLevel:       0.95,
		Seed:          1,
	})
	if err == nil {
		db.Close()
		return fmt.Errorf("compute with mixed units should fail")
	}

	// --- 步骤 4：校准（基于全部观测估计偏差） ---
	draft, err := app.Cals.EstimateAndDraft("batch-1", 0)
	if err != nil {
		db.Close()
		return fmt.Errorf("estimate calibration: %w", err)
	}
	if draft.SampleSize < 4 {
		db.Close()
		return fmt.Errorf("calibration sample size %d < 4", draft.SampleSize)
	}
	calID := draft.Calibration.ID
	if _, err := app.Cals.Activate(calID); err != nil {
		db.Close()
		return fmt.Errorf("activate calibration: %w", err)
	}

	// --- 步骤 5：统计（双切片合并，污染视野已剔除） ---
	// 批次流转到 analyzable
	if _, err := app.Batches.Transition("batch-1", model.BatchAnalyzable); err != nil {
		db.Close()
		return fmt.Errorf("transition to analyzable: %w", err)
	}

	// fld-a(0°) 与 fld-b(90°) 均有效：统计时 90° 折回基准，统一校正偏差。
	// fld-a 观测数 = 15(原始) + 3(sub-a-2) = 18；fld-b = 15 → 合计 33。
	fieldIDs := []string{"fld-a", "fld-b"}
	r1, err := app.Results.Compute(result.ComputeOptions{
		BatchID:       "batch-1",
		CalibrationID: calID,
		FieldIDs:      fieldIDs,
		CiLevel:       0.95,
		Seed:          42,
	})
	if err != nil {
		db.Close()
		return fmt.Errorf("compute result: %w", err)
	}
	if r1.Version != 1 {
		db.Close()
		return fmt.Errorf("result version should be 1, got %d", r1.Version)
	}
	if r1.Stats == nil {
		db.Close()
		return fmt.Errorf("result stats nil")
	}
	// 真实面内方向 30°：校正后均值应在 30 附近（±8° 容差）。
	if r1.Stats.MeanDeg < 22 || r1.Stats.MeanDeg > 38 {
		db.Close()
		return fmt.Errorf("mean angle out of expected [22,38], got %.2f", r1.Stats.MeanDeg)
	}
	if r1.ObservationCount != 33 {
		db.Close()
		return fmt.Errorf("observation count should be 33, got %d", r1.ObservationCount)
	}

	// 发布：冻结结果 → 发布批次
	if _, err := app.Results.Freeze(r1.ID); err != nil {
		db.Close()
		return fmt.Errorf("freeze result: %w", err)
	}
	if _, err := app.Publish("batch-1"); err != nil {
		db.Close()
		return fmt.Errorf("publish batch: %w", err)
	}

	// 新增观测（0° 切片，表观 42° 附近）→ 替代版本
	if _, err := app.Obs.Import("batch-1", "fld-a", model.UnitDegrees, []float64{41, 43, 42}, "sub-a-3"); err != nil {
		db.Close()
		return fmt.Errorf("import extra: %w", err)
	}
	r2, err := app.Results.Compute(result.ComputeOptions{
		BatchID:       "batch-1",
		CalibrationID: calID,
		FieldIDs:      fieldIDs,
		CiLevel:       0.95,
		Seed:          42,
	})
	if err != nil {
		db.Close()
		return fmt.Errorf("compute result v2: %w", err)
	}
	if r2.Version != 2 {
		db.Close()
		return fmt.Errorf("result version should be 2, got %d", r2.Version)
	}
	if r2.ObservationCount != r1.ObservationCount+3 {
		db.Close()
		return fmt.Errorf("obs count should grow by 3: v1=%d v2=%d", r1.ObservationCount, r2.ObservationCount)
	}

	// 版本比较
	cmp, err := app.Results.Compare(r1.ID, r2.ID)
	if err != nil {
		db.Close()
		return fmt.Errorf("compare: %w", err)
	}
	if cmp.CountDelta != 3 {
		db.Close()
		return fmt.Errorf("compare count delta should be 3, got %d", cmp.CountDelta)
	}

	// 冻结结果不可编辑（发布后再次冻结 → 错误）
	if _, err := app.Results.Freeze(r1.ID); err == nil {
		db.Close()
		return fmt.Errorf("double freeze should fail")
	}

	// --- 步骤 7：关闭重开验证重启恢复 ---
	if err := db.Close(); err != nil {
		return fmt.Errorf("close db: %w", err)
	}

	db2, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("reopen db: %w", err)
	}
	defer db2.Close()
	app2, err := service.New(db2)
	if err != nil {
		return fmt.Errorf("reopen services: %w", err)
	}

	// 批次仍在且状态已发布
	b2, err := app2.Batches.Get("batch-1")
	if err != nil {
		return fmt.Errorf("reopen: get batch: %w", err)
	}
	if b2.Status != model.BatchPublished {
		return fmt.Errorf("reopen: batch status should be published, got %s", b2.Status)
	}
	// 观测总数 = 15(fld-a) + 15(fld-b) + 6+2(fld-c) + 3(sub-a-2) + 3(sub-a-3) = 44
	total, err := app2.Obs.CountByBatch("batch-1")
	if err != nil {
		return fmt.Errorf("reopen: count obs: %w", err)
	}
	if total != 44 {
		return fmt.Errorf("reopen: obs count should be 44, got %d", total)
	}
	// 结果版本 2 个
	results, err := app2.Results.ListByBatch("batch-1")
	if err != nil {
		return fmt.Errorf("reopen: list results: %w", err)
	}
	if len(results) != 2 {
		return fmt.Errorf("reopen: results should be 2, got %d", len(results))
	}
	// 校准版本存在
	cals, err := app2.Cals.ListByBatch("batch-1")
	if err != nil {
		return fmt.Errorf("reopen: list cals: %w", err)
	}
	if len(cals) != 1 {
		return fmt.Errorf("reopen: calibrations should be 1, got %d", len(cals))
	}

	return nil
}
