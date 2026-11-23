package stereo

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
)

func clamp8(v float64) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 255
	}
	return uint8(v*255 + 0.5)
}

// SaveColorPNG 将颜色帧写为 PNG。
func SaveColorPNG(f *Frame, path string) error {
	img := image.NewRGBA(image.Rect(0, 0, f.Width, f.Height))
	for i, c := range f.Color {
		x, y := i%f.Width, i/f.Width
		img.SetRGBA(x, y, color.RGBA{clamp8(c.X), clamp8(c.Y), clamp8(c.Z), 255})
	}
	return writePNG(img, path)
}

// SaveDepthPNG 将深度缓冲（NDC z）灰度化输出。
func SaveDepthPNG(f *Frame, path string) error {
	img := image.NewGray(image.Rect(0, 0, f.Width, f.Height))
	for i, z := range f.Depth {
		g := clamp8((z + 1) / 2)
		x, y := i%f.Width, i/f.Width
		img.SetGray(x, y, color.Gray{Y: g})
	}
	return writePNG(img, path)
}

// SaveDiffPNG 输出放大的颜色差异图（红色通道承载总色差 ×gain）。
func SaveDiffPNG(a, b *Frame, gain float64, path string) error {
	img := image.NewRGBA(image.Rect(0, 0, a.Width, a.Height))
	for i := range a.Color {
		ca, cb := a.Color[i], b.Color[i]
		d := abs(ca.X-cb.X) + abs(ca.Y-cb.Y) + abs(ca.Z-cb.Z)
		x, y := i%a.Width, i/a.Width
		img.SetRGBA(x, y, color.RGBA{R: clamp8(d * gain), A: 255})
	}
	return writePNG(img, path)
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func writePNG(img image.Image, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}
