package main

import (
	"math"
	"math/rand"
)

// Panel 表示一块面板的逐像素相对亮度响应（1.0 为理想响应）。
// 每个通道一张图，模拟面板暗角、低频 Mura 与行列亮线的叠加结果。
type Panel struct {
	W, H int
	// Resp[c][y*W+x] 为通道 c 在 (x,y) 处的相对响应，范围大致 [0.6, 1.1]。
	Resp [3][]float64
}

// valueNoise 生成低频随机网格并做双线性平滑插值，作为低频 Mura 斑块基底。
// cells 控制网格密度，cells 越小斑块越大、频率越低。
func valueNoise(w, h, cells int, rng *rand.Rand) []float64 {
	gw, gh := cells+2, cells+2
	grid := make([]float64, gw*gh)
	for i := range grid {
		grid[i] = rng.NormFloat64()
	}
	out := make([]float64, w*h)
	for y := 0; y < h; y++ {
		// 归一化坐标映射到网格空间
		gy := float64(y) / float64(h-1) * float64(cells)
		y0 := int(gy)
		fy := gy - float64(y0)
		for x := 0; x < w; x++ {
			gx := float64(x) / float64(w-1) * float64(cells)
			x0 := int(gx)
			fx := gx - float64(x0)
			sx := fx * fx * (3 - 2*fx)
			sy := fy * fy * (3 - 2*fy)
			v00 := grid[y0*gw+x0]
			v10 := grid[y0*gw+x0+1]
			v01 := grid[(y0+1)*gw+x0]
			v11 := grid[(y0+1)*gw+x0+1]
			top := v00 + (v10-v00)*sx
			bot := v01 + (v11-v01)*sx
			out[y*w+x] = top + (bot-top)*sy
		}
	}
	return out
}

// NewPanel 合成面板响应：低频 Mura + 随机行列亮线 + 四角暗角衰减。
func NewPanel(w, h int, seed int64) *Panel {
	rng := rand.New(rand.NewSource(seed))
	p := &Panel{W: w, H: h}

	// 各通道独立的低频 Mura（两个倍频叠加，模拟不同尺度的斑块）
	mura := [3][]float64{}
	for c := 0; c < 3; c++ {
		base := valueNoise(w, h, 6, rng)
		fine := valueNoise(w, h, 18, rng)
		m := make([]float64, w*h)
		for i := range m {
			m[i] = base[i]*0.030 + fine[i]*0.012
		}
		mura[c] = m
	}

	// 随机行亮线（整行偏亮）与列亮线
	rowGain := make([]float64, h)
	for i := 0; i < 7; i++ {
		rowGain[rng.Intn(h)] += 0.02 + rng.Float64()*0.03
	}
	colGain := make([]float64, w)
	for i := 0; i < 7; i++ {
		colGain[rng.Intn(w)] += 0.02 + rng.Float64()*0.03
	}

	// 四角衰减（暗角）：以到中心的归一化距离构造径向衰减
	cx, cy := float64(w-1)/2, float64(h-1)/2
	maxR := math.Hypot(cx, cy)
	vign := make([]float64, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r := math.Hypot(float64(x)-cx, float64(y)-cy) / maxR
			// 中心平坦、四角加速下坠的余弦型衰减
			vign[y*w+x] = -0.18 * math.Pow(math.Sin(r*math.Pi/2), 4)
		}
	}

	for c := 0; c < 3; c++ {
		resp := make([]float64, w*h)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				i := y*w + x
				resp[i] = 1.0 + mura[c][i] + rowGain[y] + colGain[x] + vign[i]
			}
		}
		p.Resp[c] = resp
	}
	return p
}
