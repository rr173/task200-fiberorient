// Package statistics 实现纤维取向的圆周（轴向）统计。
//
// 纤维取向是轴向数据：角度以 180° 为模，0° 与 180° 代表同一方向。标准做法是
// 把每个角度翻倍（doubling），在 [0,360) 上按方向数据（circular data）计算均值与
// 合矢量，最后把结果折回 [0,180)。本包同时提供 Rayleigh 各向同性检验、双峰检测
// 与均值置信区间。
package statistics

import (
	"math"
	"sort"
)

const (
	pi      = math.Pi
	tau     = 2 * math.Pi
	deg2rad = pi / 180
	rad2deg = 180 / pi
)

// Sample 待统计的纤维角度（单位：度，已归一化到 [0,180)）。
type Sample struct {
	AngleDeg float64
}

// CircularStats 计算轴向样本的圆周统计量。
func CircularStats(angles []float64) (meanDeg, R, variance, kappa, rayleighP float64) {
	n := len(angles)
	if n == 0 {
		return 0, 0, 1, 0, 1
	}
	var sx, sy float64
	for _, a := range angles {
		th := a * deg2rad
		sx += math.Cos(2 * th) // 轴向：翻倍
		sy += math.Sin(2 * th)
	}
	mx := sx / float64(n)
	my := sy / float64(n)
	R = math.Hypot(mx, my)
	variance = 1 - R
	meanRad := 0.5 * math.Atan2(my, mx) // 折回轴向
	if meanRad < 0 {
		meanRad += pi
	}
	meanDeg = meanRad * rad2deg

	// von Mises 集中度 kappa 的近似估计（Fisher 公式）。
	if R < 0.53 {
		kappa = 2*R + R*R*R + 5.0/6.0*math.Pow(R, 5)
	} else if R < 0.85 {
		kappa = -0.4 + 1.39*R + 0.43/(1-R)
	} else {
		kappa = 1.0 / (R*R*R - 4*R*R + 3*R)
	}
	if math.IsInf(kappa, 0) || math.IsNaN(kappa) {
		kappa = 1000
	}

	// Rayleigh 检验：z = n * R²（轴向数据 z 不变，R 为轴向合矢量）。
	z := float64(n) * R * R
	rayleighP = math.Exp(-z)
	if rayleighP > 1 {
		rayleighP = 1
	}
	return meanDeg, R, variance, kappa, rayleighP
}

// CorrectedAngles 把原始角度按偏差补偿并归一化到 [0,180)。
func CorrectedAngles(angles []float64, biasDeg float64) []float64 {
	out := make([]float64, len(angles))
	for i, a := range angles {
		c := a + biasDeg
		c = math.Mod(c, 180)
		if c < 0 {
			c += 180
		}
		out[i] = c
	}
	return out
}

// BimodalityResult 双峰检测结果。
type BimodalityResult struct {
	IsBimodal     bool
	PrimaryDeg    float64
	SecondaryDeg  float64
	DipStatistic  float64
	SeparationDeg float64
}

// DetectBimodality 检测轴向分布是否存在两个分离的方向峰。
//
// 方法：把样本翻倍后做直方图（18 个 20° 桶，对应原角度 10° 桶），对桶频数做
// 平滑，寻找两个局部极大；若两峰间距 ≥ 45° 且峰谷比 dip 显著（谷值/均值 ≤ 0.6），
// 判定为双峰。返回两峰位置（已折回 [0,180)）。
func DetectBimodality(angles []float64) BimodalityResult {
	const buckets = 18
	n := len(angles)
	if n < 8 {
		return BimodalityResult{}
	}
	hist := make([]int, buckets)
	for _, a := range angles {
		idx := int(math.Mod(a*2, 360)) / (360 / buckets)
		hist[idx]++
	}
	// 平滑（环形三邻域）。
	sm := make([]float64, buckets)
	for i := range hist {
		sm[i] = float64(hist[(i-1+buckets)%buckets]+hist[i]+hist[(i+1)%buckets]) / 3.0
	}
	// 找局部极大。
	var peaks []int
	for i := range sm {
		prev := sm[(i-1+buckets)%buckets]
		next := sm[(i+1)%buckets]
		if sm[i] >= prev && sm[i] >= next && sm[i] > 0 {
			peaks = append(peaks, i)
		}
	}
	// 合并邻近峰（间距 < 3 桶）。
	var merged []int
	for _, p := range peaks {
		if len(merged) > 0 && p-merged[len(merged)-1] <= 3 {
			continue
		}
		merged = append(merged, p)
	}
	if len(merged) < 2 {
		return BimodalityResult{}
	}
	// 取最高的两个峰（环形间距至少 4 桶 = 原角度 20°）。
	best := []int{merged[0], merged[1]}
	bestScore := float64(hist[best[0]] + hist[best[1]])
	for i := 0; i < len(merged); i++ {
		for j := i + 1; j < len(merged); j++ {
			d := ringDist(merged[i], merged[j], buckets)
			if d < 4 {
				continue
			}
			score := float64(hist[merged[i]] + hist[merged[j]])
			if score > bestScore {
				best = []int{merged[i], merged[j]}
				bestScore = score
			}
		}
	}
	a, b := best[0], best[1]
	// 峰谷：两峰之间最小值。
	lo, hi := a, b
	if hi < lo {
		lo, hi = hi, lo
	}
	valley := sm[lo]
	for k := lo; k <= hi; k++ {
		if sm[k] < valley {
			valley = sm[k]
		}
	}
	mean := 0.0
	for _, v := range sm {
		mean += v
	}
	mean /= float64(buckets)
	ratio := 1.0
	if mean > 0 {
		ratio = valley / mean
	}
	dip := valley / mean
	sepBuckets := ringDist(a, b, buckets)
	sep := float64(sepBuckets) * (180.0 / float64(buckets)) // 翻倍域间距折回轴向为半距
	sepDeg := sep / 2

	primaryDeg := float64(a) * (180.0 / float64(buckets))
	secondaryDeg := float64(b) * (180.0 / float64(buckets))
	isBimodal := ratio <= 0.6 && sepDeg >= 45
	if primaryDeg > secondaryDeg {
		primaryDeg, secondaryDeg = secondaryDeg, primaryDeg
	}
	return BimodalityResult{
		IsBimodal:     isBimodal,
		PrimaryDeg:    primaryDeg,
		SecondaryDeg:  secondaryDeg,
		DipStatistic:  dip,
		SeparationDeg: sepDeg,
	}
}

func ringDist(a, b, n int) int {
	d := a - b
	if d < 0 {
		d = -d
	}
	if d > n/2 {
		d = n - d
	}
	return d
}

// ConfidenceInterval 计算均值置信区间（bootstrap 百分位法，可复现）。
func ConfidenceInterval(angles []float64, level float64, seed int64) (lower, upper float64) {
	n := len(angles)
	if n < 4 {
		return 0, 180
	}
	if level <= 0 || level >= 1 {
		level = 0.95
	}
	rng := newRand(seed)
	const replicates = 1999
	means := make([]float64, 0, replicates)
	for i := 0; i < replicates; i++ {
		var sx, sy float64
		for j := 0; j < n; j++ {
			th := angles[rng.Intn(n)] * deg2rad
			sx += math.Cos(2 * th)
			sy += math.Sin(2 * th)
		}
		mx := sx / float64(n)
		my := sy / float64(n)
		m := 0.5 * math.Atan2(my, mx)
		if m < 0 {
			m += pi
		}
		means = append(means, m*rad2deg)
	}
	sort.Float64s(means)
	alpha := 1 - level
	loIdx := int(math.Floor(alpha / 2 * float64(len(means))))
	hiIdx := int(math.Ceil((1-alpha/2)*float64(len(means)))) - 1
	if loIdx < 0 {
		loIdx = 0
	}
	if hiIdx >= len(means) {
		hiIdx = len(means) - 1
	}
	return means[loIdx], means[hiIdx]
}
