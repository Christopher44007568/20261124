package main

// maxGain 限制补偿增益上限，避免对极暗角落过度放大而引入噪声/色偏。
const maxGain = 1.6

// CompMap 是通道级补偿增益图，与面板同分辨率。
type CompMap struct {
	W, H int
	Gain [3][]float64
}

// BuildComp 由面板响应计算补偿图。
// 原则：只抬升暗区、不压低亮区——以全屏最大响应（通常即中心峰值）为基准，
// 增益恒 >= 1，中心峰值保持不变，不会被为了均匀性而压低。
func BuildComp(p *Panel) *CompMap {
	cm := &CompMap{W: p.W, H: p.H}
	for c := 0; c < 3; c++ {
		peak := 0.0
		for _, r := range p.Resp[c] {
			if r > peak {
				peak = r
			}
		}
		g := make([]float64, p.W*p.H)
		for i, r := range p.Resp[c] {
			gain := peak / r
			if gain < 1.0 {
				gain = 1.0
			}
			if gain > maxGain {
				gain = maxGain
			}
			g[i] = gain
		}
		cm.Gain[c] = g
	}
	return cm
}

// clamp 将坐标钳制到 [0, limit]。
func clamp(v, limit float64) float64 {
	if v < 0 {
		return 0
	}
	if v > limit {
		return limit
	}
	return v
}

// Sample 对通道 c 的补偿图做双线性采样。
// 屏幕外坐标一律钳制到边缘像素，绝不环绕/重复采样补偿纹理，
// 避免边缘补偿值被错误地平铺到屏外区域。
func (cm *CompMap) Sample(c int, x, y float64) float64 {
	x = clamp(x, float64(cm.W-1))
	y = clamp(y, float64(cm.H-1))
	x0 := int(x)
	y0 := int(y)
	x1 := x0 + 1
	if x1 > cm.W-1 {
		x1 = cm.W - 1
	}
	y1 := y0 + 1
	if y1 > cm.H-1 {
		y1 = cm.H - 1
	}
	fx := x - float64(x0)
	fy := y - float64(y0)
	g := cm.Gain[c]
	w := cm.W
	v00 := g[y0*w+x0]
	v10 := g[y0*w+x1]
	v01 := g[y1*w+x0]
	v11 := g[y1*w+x1]
	top := v00 + (v10-v00)*fx
	bot := v01 + (v11-v01)*fx
	return top + (bot-top)*fy
}

// Apply 用补偿图调制输入像素（0..1 浮点），再经面板响应得到最终显示亮度。
// 返回结果可能超过 1（增益被 maxGain 截断时角落仍偏暗），调用方负责截断显示。
func Apply(cm *CompMap, p *Panel, c, x, y int, in float64) float64 {
	g := cm.Sample(c, float64(x), float64(y))
	return in * g * p.Resp[c][y*p.W+x]
}

