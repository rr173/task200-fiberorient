package statistics

import (
	"math"
	"testing"
)

func TestCircularStatsUniform(t *testing.T) {
	// 均匀分布：R 应接近 0，p 值接近 1。
	var angles []float64
	for i := 0; i < 200; i++ {
		angles = append(angles, float64(i%180))
	}
	mean, R, variance, kappa, p := CircularStats(angles)
	if R > 0.15 {
		t.Fatalf("uniform R should be small, got %f", R)
	}
	if variance < 0.85 {
		t.Fatalf("uniform variance should be large, got %f", variance)
	}
	if p < 0.05 {
		t.Fatalf("uniform Rayleigh p should be large, got %f", p)
	}
	if kappa < 0 {
		t.Fatalf("kappa negative: %f", kappa)
	}
	_ = mean
}

func TestCircularStatsConcentrated(t *testing.T) {
	// 集中在 30° 附近：均值应接近 30，R 接近 1。
	angles := []float64{28, 31, 29, 33, 30, 27, 32, 30, 29, 31, 30, 28, 33, 30, 31, 29, 30, 32, 30, 28}
	mean, R, variance, _, p := CircularStats(angles)
	if math.Abs(mean-30) > 2 {
		t.Fatalf("mean should be near 30, got %f", mean)
	}
	if R < 0.9 {
		t.Fatalf("R should be high, got %f", R)
	}
	if variance > 0.1 {
		t.Fatalf("variance should be low, got %f", variance)
	}
	if p > 0.01 {
		t.Fatalf("concentrated Rayleigh p should be tiny, got %f", p)
	}
}

func TestCorrectedAngles(t *testing.T) {
	got := CorrectedAngles([]float64{10, 170, 90}, 20)
	want := []float64{30, 10, 110}
	for i := range got {
		if math.Abs(got[i]-want[i]) > 1e-9 {
			t.Fatalf("corrected[%d]=%f want %f", i, got[i], want[i])
		}
	}
}

func TestCorrectedAnglesWrap(t *testing.T) {
	got := CorrectedAngles([]float64{179, 178}, 3)
	if math.Abs(got[0]-2) > 1e-9 || math.Abs(got[1]-1) > 1e-9 {
		t.Fatalf("wrap correction wrong: %v", got)
	}
}

func TestDetectBimodality(t *testing.T) {
	// 双峰：30° 与 120° 各 60 个。
	angles := []float64{}
	for i := 0; i < 60; i++ {
		angles = append(angles, 30+float64(i%5))
	}
	for i := 0; i < 60; i++ {
		angles = append(angles, 120+float64(i%5))
	}
	b := DetectBimodality(angles)
	if !b.IsBimodal {
		t.Fatalf("should detect bimodal, got %+v", b)
	}
	if b.SeparationDeg < 45 {
		t.Fatalf("separation should be >= 45, got %f", b.SeparationDeg)
	}
}

func TestDetectBimodalitySingle(t *testing.T) {
	angles := []float64{}
	for i := 0; i < 120; i++ {
		angles = append(angles, 30+float64(i%4))
	}
	b := DetectBimodality(angles)
	if b.IsBimodal {
		t.Fatalf("single peak should not be bimodal, got %+v", b)
	}
}

func TestEstimateBias(t *testing.T) {
	// 真实面内方向 40°，仪器偏差 +15°：
	// 0° 切片表观 θ0 = 40+15 = 55° 附近；
	// 90° 切片表观 θ90 = 40+90−15 = 115° 附近。
	raw0 := []float64{55, 56, 54, 57, 55, 53, 56, 55, 54, 57, 55, 56}
	raw90 := []float64{115, 116, 114, 117, 115, 113, 116, 115, 114, 117, 115, 116}
	bias, conf := EstimateBias(raw0, raw90)
	if math.Abs(bias-15) > 3 {
		t.Fatalf("estimated bias %f should be near 15", bias)
	}
	if conf < 0.5 || conf > 1 {
		t.Fatalf("confidence out of range: %f", conf)
	}
}

func TestCorrectForSlice(t *testing.T) {
	// 真实 φ=30，δ=12：
	// 0° 切片 42° → 校正 30；90° 切片 108° → 校正 30。
	if got := CorrectForSlice(42, 0, 12); math.Abs(got-30) > 1e-9 {
		t.Fatalf("0° slice correct: got %f want 30", got)
	}
	if got := CorrectForSlice(108, 90, 12); math.Abs(got-30) > 1e-9 {
		t.Fatalf("90° slice correct: got %f want 30", got)
	}
}

func TestConfidenceIntervalReproducible(t *testing.T) {
	angles := []float64{28, 31, 29, 33, 30, 27, 32, 30, 29, 31, 30, 28, 33, 30, 31, 29, 30, 32}
	l1, u1 := ConfidenceInterval(angles, 0.95, 7)
	l2, u2 := ConfidenceInterval(angles, 0.95, 7)
	if l1 != l2 || u1 != u2 {
		t.Fatalf("CI not reproducible: (%f,%f) vs (%f,%f)", l1, u1, l2, u2)
	}
	if l1 > u1 {
		t.Fatalf("CI inverted: lower %f > upper %f", l1, u1)
	}
}

func TestRngIntn(t *testing.T) {
	r := newRand(42)
	seen := map[int]bool{}
	for i := 0; i < 1000; i++ {
		n := r.Intn(10)
		if n < 0 || n >= 10 {
			t.Fatalf("Intn out of range: %d", n)
		}
		seen[n] = true
	}
	if len(seen) < 5 {
		t.Fatalf("Intn too narrow: %d distinct", len(seen))
	}
}
