package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

const (
	screenW = 640
	screenH = 360
	seed    = 20261124
	outDir  = "artifacts"
)

// render 用给定电平函数渲染一帧：comp 为 nil 时表示不补偿。
// 返回 8bit 图像与每通道浮点亮度（用于统计）。
func render(p *Panel, cm *CompMap, pat Pattern) (*image.RGBA, [3][]float64) {
	img := image.NewRGBA(image.Rect(0, 0, p.W, p.H))
	var lum [3][]float64
	for c := 0; c < 3; c++ {
		lum[c] = make([]float64, p.W*p.H)
	}
	for y := 0; y < p.H; y++ {
		for x := 0; x < p.W; x++ {
			in := pat(x, y, p.W, p.H)
			var px [3]float64
			for c := 0; c < 3; c++ {
				v := in[c] * p.Resp[c][y*p.W+x]
				if cm != nil {
					v = in[c] * cm.Sample(c, float64(x), float64(y)) * p.Resp[c][y*p.W+x]
				}
				px[c] = v
				lum[c][y*p.W+x] = v
			}
			img.SetRGBA(x, y, color.RGBA{
				R: to8(px[0]),
				G: to8(px[1]),
				B: to8(px[2]),
				A: 255,
			})
		}
	}
	return img, lum
}

func to8(v float64) uint8 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 255
	}
	return uint8(math.Round(v * 255))
}

// stats 报告单通道亮度均匀性：均值、标准差、变异系数、最小/最大。
func stats(name string, lum []float64) {
	n := float64(len(lum))
	var sum, min, max float64
	min = math.Inf(1)
	max = math.Inf(-1)
	for _, v := range lum {
		sum += v
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	mean := sum / n
	var sq float64
	for _, v := range lum {
		d := v - mean
		sq += d * d
	}
	std := math.Sqrt(sq / n)
	cv := 0.0
	if mean > 0 {
		cv = std / mean * 100
	}
	fmt.Printf("  %-7s mean=%.4f std=%.4f cv=%5.2f%% min=%.4f max=%.4f\n",
		name, mean, std, cv, min, max)
}

// activeChannel 返回平均亮度最高的通道下标，用于纯色画面只统计有效通道。
func activeChannel(lum [3][]float64) int {
	best, bestSum := 0, -1.0
	for c := 0; c < 3; c++ {
		var s float64
		for _, v := range lum[c] {
			s += v
		}
		if s > bestSum {
			best, bestSum = c, s
		}
	}
	return best
}

func savePNG(img *image.RGBA, name string) {
	path := filepath.Join(outDir, name)
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		panic(err)
	}
}

func main() {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		panic(err)
	}
	panel := NewPanel(screenW, screenH, seed)
	comp := BuildComp(panel)

	// 中心峰值保护自检：补偿后中心响应不得被压低
	cx, cy := screenW/2, screenH/2
	for c := 0; c < 3; c++ {
		g := comp.Sample(c, float64(cx), float64(cy))
		fmt.Printf("center gain ch%d = %.4f (must be >= 1)\n", c, g)
	}
	// 屏外采样自检：负坐标与超界坐标应钳制到边缘值而非环绕
	edge := comp.Sample(0, 0, 0)
	fmt.Printf("edge clamp check: sample(-50,-50)=%.4f edge(0,0)=%.4f (must match)\n",
		comp.Sample(0, -50, -50), edge)

	patterns := []struct {
		name string
		pat  Pattern
	}{
		{"black", patternBlack},
		{"gray17", patternGray17},
		{"red", pureColor(0)},
		{"green", pureColor(1)},
		{"blue", pureColor(2)},
		{"gradient", patternHighlightGradient},
	}
	for _, pt := range patterns {
		fmt.Printf("pattern %s:\n", pt.name)
		imgRaw, lumRaw := render(panel, nil, pt.pat)
		imgFix, lumFix := render(panel, comp, pt.pat)
		savePNG(imgRaw, pt.name+"_raw.png")
		savePNG(imgFix, pt.name+"_comp.png")
		c := activeChannel(lumRaw)
		ch := string("RGB"[c])
		stats("raw "+ch, lumRaw[c])
		stats("comp "+ch, lumFix[c])
	}
	fmt.Println("done ->", outDir)
}
