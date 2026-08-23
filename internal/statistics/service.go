package statistics

import (
	"math"

	"task200-fiberorient/internal/model"
)

// Compute 对一组已校正的归一化角度执行完整圆周统计：
// 均值/合矢量/方差/kappa/Rayleigh p、双峰检测与置信区间。
func Compute(angles []float64, ciLevel float64, seed int64) (*model.CircularStats, *model.Bimodality, *model.ConfidenceInterval) {
	mean, R, variance, kappa, p := CircularStats(angles)

	stats := &model.CircularStats{
		MeanDeg:          mean,
		ResultantLength:  R,
		CircularVariance: variance,
		SampleSize:       len(angles),
		Kappa:            kappa,
		RayleighP:        p,
	}

	bim := DetectBimodality(angles)
	bimodal := &model.Bimodality{
		IsBimodal:     bim.IsBimodal,
		PrimaryDeg:    bim.PrimaryDeg,
		SecondaryDeg:  bim.SecondaryDeg,
		DipStatistic:  bim.DipStatistic,
		SeparationDeg: bim.SeparationDeg,
	}

	lower, upper := ConfidenceInterval(angles, ciLevel, seed)
	ci := &model.ConfidenceInterval{
		LowerDeg: lower,
		UpperDeg: upper,
		Level:    ciLevel,
		Method:   "bootstrap_percentile",
	}
	return stats, bimodal, ci
}

// EstimateBias 用 0°/90° 两个正交切片的观测估计仪器切片偏差 δ。
//
// 物理模型：纸样面内纤维取向为 φ（轴向，模 180°）；正交切片对偏差响应符号相反：
// 0° 切片表观 θ0 = φ + δ，90° 切片表观 θ90 = φ + 90 − δ（mod 180）。
// 把 90° 组折回基准坐标系（θ90 − 90）后，两组均值之差的一半即 δ：
//
//	δ = norm180(mean(θ0) − mean(θ90 − 90)) / 2
//
// 切片偏差物理上不超过 90°，返回 δ ∈ [0,90)。
func EstimateBias(angles0, angles90 []float64) (biasDeg, confidence float64) {
	if len(angles0) < 3 || len(angles90) < 3 {
		return 0, 0
	}
	aligned90 := make([]float64, len(angles90))
	for i, a := range angles90 {
		aligned90[i] = norm180(a - 90)
	}
	mean0, R0, _, _, _ := CircularStats(angles0)
	mean90, R90, _, _, _ := CircularStats(aligned90)
	delta := norm180(mean0-mean90) / 2
	if delta >= 90 {
		delta -= 90
	}
	conf := 0.5 + 0.49*((R0+R90)/2)
	if conf > 0.99 {
		conf = 0.99
	}
	return delta, conf
}

// CorrectForSlice 按切片方向校正表观角为面内取向：
//   - 0° 切片：θ − δ
//   - 90° 切片：先折回基准再 +δ，即 (θ − 90) + δ
func CorrectForSlice(angleDeg, sliceDeg, biasDeg float64) float64 {
	if sliceDeg == 90 {
		return norm180(angleDeg - 90 + biasDeg)
	}
	return norm180(angleDeg - biasDeg)
}

// norm180 把角度归一化到 [0,180)。
func norm180(a float64) float64 {
	a = math.Mod(a, 180)
	if a < 0 {
		a += 180
	}
	return a
}
