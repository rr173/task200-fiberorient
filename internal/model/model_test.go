package model

import "testing"

func TestBatchStateMachine(t *testing.T) {
	b, err := NewBatch("b1", "测试批", "纸浆", 45)
	if err != nil {
		t.Fatal(err)
	}
	if b.Status != BatchRegistered {
		t.Fatalf("initial status should be registered, got %s", b.Status)
	}
	if err := b.Transition(BatchObserving); err != nil {
		t.Fatalf("registered->observing should pass: %v", err)
	}
	if err := b.Transition(BatchAnalyzable); err != nil {
		t.Fatalf("observing->analyzable should pass: %v", err)
	}
	if err := b.Transition(BatchPublished); err != nil {
		t.Fatalf("analyzable->published should pass: %v", err)
	}
	// 已发布不允许回到 analyzable
	if err := b.Transition(BatchAnalyzable); err == nil {
		t.Fatal("published->analyzable should fail")
	}
}

func TestBatchInvalidSliceAngle(t *testing.T) {
	if _, err := NewBatch("b2", "批", "浆", 180); err == nil {
		t.Fatal("slice angle 180 should be rejected")
	}
	if _, err := NewBatch("b3", "批", "浆", -1); err == nil {
		t.Fatal("negative slice angle should be rejected")
	}
}

func TestFieldStateMachine(t *testing.T) {
	f, err := NewField("f1", "b1", "视野1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if f.Status != FieldPending {
		t.Fatalf("initial status should be pending, got %s", f.Status)
	}
	if err := f.MarkPolluted("脏污"); err != nil {
		t.Fatalf("mark polluted: %v", err)
	}
	if f.Status != FieldPolluted {
		t.Fatalf("should be polluted, got %s", f.Status)
	}
	if err := f.MarkValid(); err != nil {
		t.Fatalf("polluted->valid: %v", err)
	}
	if err := f.Exclude(); err != nil {
		t.Fatalf("exclude: %v", err)
	}
	// excluded 是终态
	if err := f.MarkValid(); err == nil {
		t.Fatal("excluded->valid should fail")
	}
}

func TestObservationNormalize(t *testing.T) {
	o, err := NewObservation("o1", "b1", "f1", 200, UnitDegrees, "sub-1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if o.AngleDeg != 20 {
		t.Fatalf("200° should normalize to 20°, got %f", o.AngleDeg)
	}
	// 指纹按提交批次+序号：同提交同序号幂等，与角度值无关
	a, _ := NewObservation("o2", "b1", "f1", 30, UnitDegrees, "sub-1", 0)
	b, _ := NewObservation("o3", "b1", "f1", 30, UnitDegrees, "sub-1", 0)
	if a.Fingerprint != b.Fingerprint {
		t.Fatalf("same submission+seq should share fingerprint: %s vs %s", a.Fingerprint, b.Fingerprint)
	}
	c, _ := NewObservation("o4", "b1", "f1", 30, UnitDegrees, "sub-2", 0)
	if a.Fingerprint == c.Fingerprint {
		t.Fatalf("different submission should differ: %s vs %s", a.Fingerprint, c.Fingerprint)
	}
}

func TestObservationUnitRejection(t *testing.T) {
	if _, err := NewObservation("o5", "b1", "f1", 1.0, UnitDegrees, "sub-1", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := NewObservation("o6", "b1", "f1", 6.3, UnitRadians, "sub-1", 0); err == nil {
		t.Fatal("6.3 rad should be rejected (>2π)")
	}
	if _, err := NewObservation("o7", "b1", "f1", 1.0, "bogus", "sub-1", 0); err == nil {
		t.Fatal("bogus unit should be rejected")
	}
	if _, err := NewObservation("o8", "b1", "f1", 1.0, UnitDegrees, "", 0); err == nil {
		t.Fatal("empty submission id should be rejected")
	}
}

func TestCalibrationLifecycle(t *testing.T) {
	c, err := NewCalibration("c1", "b1", 12.5, 30, 0.8)
	if err != nil {
		t.Fatal(err)
	}
	if c.Status != CalibrationDraft {
		t.Fatalf("should start draft, got %s", c.Status)
	}
	if err := c.Activate(); err != nil {
		t.Fatal(err)
	}
	// 激活后不能再激活
	if err := c.Activate(); err == nil {
		t.Fatal("double activate should fail")
	}
	if err := c.Revoke(); err != nil {
		t.Fatal(err)
	}
	// 废止后不能再废止
	if err := c.Revoke(); err == nil {
		t.Fatal("double revoke should fail")
	}
}

func TestResultFinalizeAndFreeze(t *testing.T) {
	r, err := NewResult("r1", "b1", 1, "c1", "f1,f2", 20)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != ResultComputing {
		t.Fatalf("should start computing, got %s", r.Status)
	}
	stats := &CircularStats{MeanDeg: 30, ResultantLength: 0.95, CircularVariance: 0.05, SampleSize: 20, Kappa: 20, RayleighP: 0.001}
	ci := &ConfidenceInterval{LowerDeg: 28, UpperDeg: 32, Level: 0.95, Method: "bootstrap"}
	if err := r.Finalize(stats, nil, ci, 45); err != nil {
		t.Fatal(err)
	}
	if r.Status != ResultPublishable {
		t.Fatalf("narrow CI should be publishable, got %s", r.Status)
	}
	if err := r.Freeze(); err != nil {
		t.Fatal(err)
	}
	if r.Status != ResultFrozen {
		t.Fatalf("should be frozen, got %s", r.Status)
	}
	// Freeze 必须写入冻结时间戳，供 store 层原样持久化。
	if r.FrozenAt == nil {
		t.Fatal("freeze should stamp FrozenAt, got nil")
	}
	// 冻结后不可再 freeze
	if err := r.Freeze(); err == nil {
		t.Fatal("double freeze should fail")
	}
}

func TestResultInsufficientConfidence(t *testing.T) {
	r, err := NewResult("r2", "b1", 2, "c1", "f1", 5)
	if err != nil {
		t.Fatal(err)
	}
	stats := &CircularStats{MeanDeg: 30, ResultantLength: 0.4, CircularVariance: 0.6, SampleSize: 5, Kappa: 1, RayleighP: 0.4}
	ci := &ConfidenceInterval{LowerDeg: 5, UpperDeg: 120, Level: 0.95, Method: "bootstrap"}
	if err := r.Finalize(stats, nil, ci, 45); err != nil {
		t.Fatal(err)
	}
	if r.Status != ResultInsufficientConf {
		t.Fatalf("wide CI should be insufficient, got %s", r.Status)
	}
	// insufficient 不可直接冻结
	if err := r.Freeze(); err == nil {
		t.Fatal("freeze insufficient should fail")
	}
}
