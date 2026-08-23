package statistics

// 可复现的伪随机数生成器（线性同余），用于 bootstrap 重采样。
// 不依赖 math/rand 全局状态，保证同一 seed 得到一致结果（可复现性）。

type lcg struct {
	state uint64
}

const lcgA = 6364136223846793005
const lcgC = 1442695040888963407

func newRand(seed int64) *lcg {
	s := uint64(seed)
	if s == 0 {
		s = 0x9E3779B97F4A7C15
	}
	return &lcg{state: s}
}

func (r *lcg) next() uint64 {
	r.state = r.state*lcgA + lcgC
	return r.state
}

// Intn 返回 [0,n) 内的整数。
func (r *lcg) Intn(n int) int {
	if n <= 0 {
		return 0
	}
	return int(r.next() % uint64(n))
}
