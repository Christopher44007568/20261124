package main

// Pattern 返回像素 (x,y) 处各通道的理想输入电平（0..1）。
type Pattern func(x, y, w, h int) [3]float64

// patternBlack 全黑画面：检验补偿是否会在黑电平引入噪底。
func patternBlack(x, y, w, h int) [3]float64 {
	return [3]float64{0, 0, 0}
}

// patternGray17 17% 灰：Mura 与暗角在该电平最容易被肉眼察觉。
func patternGray17(x, y, w, h int) [3]float64 {
	return [3]float64{0.17, 0.17, 0.17}
}

// pureColor 生成纯色画面（单通道满电平）。
func pureColor(c int) Pattern {
	return func(x, y, w, h int) [3]float64 {
		var v [3]float64
		v[c] = 1.0
		return v
	}
}

// patternHighlightGradient 高亮渐变：左到右 0.5 -> 1.0，
// 检验补偿在高电平下是否削顶、以及通道间是否引入色偏。
func patternHighlightGradient(x, y, w, h int) [3]float64 {
	v := 0.5 + 0.5*float64(x)/float64(w-1)
	return [3]float64{v, v, v}
}
