package stereo

import (
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

func SavePNG(path string, img *image.RGBA) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return png.Encode(file, img)
}

func SaveDepthPNG(path string, depth []float64) error {
	img := image.NewRGBA(image.Rect(0, 0, ImageWidth, ImageHeight))
	const nearZ = MinSceneZ
	const farZ = WallZ + 50
	for idx, z := range depth {
		if math.IsInf(z, 0) || math.IsNaN(z) || z <= 0 {
			writePixel(img, idx, color.RGBA{A: 255})
			continue
		}
		t := clamp01((z - nearZ) / (farZ - nearZ))
		value := uint8(math.Round(255 * (1 - 0.88*t)))
		writePixel(img, idx, color.RGBA{R: value, G: value, B: value, A: 255})
	}
	return SavePNG(path, img)
}

func SaveJSON(path string, value any) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
