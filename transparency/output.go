package transparency

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
)

// WriteJSON writes the machine-readable report. Frame buffers are excluded
// from JSON; images are written separately by WriteComparisonImages.
func WriteJSON(path string, evaluation *Evaluation) error {
	data, err := json.MarshalIndent(evaluation, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

// WriteTextReport prints the aggregate comparison.
func WriteTextReport(w io.Writer, evaluation *Evaluation) {
	fmt.Fprintf(w, "draw order (far -> near): %v\n", evaluation.DrawOrder)
	for _, layer := range evaluation.Layers {
		fmt.Fprintf(w, "  %-9s depth=%5.2f  history_weight=%.3f\n",
			layer.ID, layer.Depth, layer.HistoryWeight)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "strategy      color_rmse  alpha_mae  coverage_iou  order_agree  stale_coverage")
	for _, name := range []string{StrategyPerLayer, StrategyWholeBlock, StrategyNoHistory} {
		summary := evaluation.Summary[name]
		m := summary.Metrics
		fmt.Fprintf(w, "%-13s %10.5f  %9.5f  %12.5f  %11.5f  %14.5f\n",
			name, m.ColorRMSE, m.AlphaMAE, m.CoverageIoU, m.OrderAgreement, m.StaleCoverage)
	}
	block := evaluation.Summary[StrategyWholeBlock]
	fmt.Fprintf(w, "\nwhole-block shared history weight: %.3f\n", block.SharedWeight)
}

// WriteComparisonImages writes color and near-coverage PNGs for the reference
// and all three strategies.
func WriteComparisonImages(dir string, run FrameRun, config Config, scale int) error {
	if scale <= 0 {
		scale = 1
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	images := map[string]FrameResult{StrategyReference: run.Reference}
	for name, result := range run.Results {
		images[name] = result
	}
	for name, result := range images {
		colorPath := filepath.Join(dir, name+"-color.png")
		if err := writeColorPNG(colorPath, result, config, scale); err != nil {
			return err
		}
		coveragePath := filepath.Join(dir, name+"-coverage.png")
		if err := writeCoveragePNG(coveragePath, result, config, scale); err != nil {
			return err
		}
	}
	return nil
}

const StrategyReference = "reference"

func writeColorPNG(path string, frame FrameResult, config Config, scale int) error {
	img := image.NewRGBA(image.Rect(0, 0, config.Width*scale, config.Height*scale))
	for y := 0; y < config.Height; y++ {
		for x := 0; x < config.Width; x++ {
			c := frame.RGB[y*config.Width+x]
			rgba := color.RGBA{
				R: encodeChannel(c.R),
				G: encodeChannel(c.G),
				B: encodeChannel(c.B),
				A: 255,
			}
			fillScaled(img, x, y, scale, rgba)
		}
	}
	return writePNG(path, img)
}

func writeCoveragePNG(path string, frame FrameResult, config Config, scale int) error {
	img := image.NewRGBA(image.Rect(0, 0, config.Width*scale, config.Height*scale))
	for y := 0; y < config.Height; y++ {
		for x := 0; x < config.Width; x++ {
			v := encodeChannel(frame.NearAlpha[y*config.Width+x])
			rgba := color.RGBA{R: v, G: v, B: v, A: 255}
			fillScaled(img, x, y, scale, rgba)
		}
	}
	return writePNG(path, img)
}

func fillScaled(img *image.RGBA, x, y, scale int, c color.RGBA) {
	for dy := 0; dy < scale; dy++ {
		for dx := 0; dx < scale; dx++ {
			img.SetRGBA(x*scale+dx, y*scale+dy, c)
		}
	}
}

func encodeChannel(v float64) uint8 {
	v = math.Pow(clamp(v, 0, 1), 1/2.2)
	return uint8(math.Round(v * 255))
}

func writePNG(path string, img image.Image) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(file, img); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}
