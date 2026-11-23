package reprojlab

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
)

func WriteExperimentArtifacts(dir string, cfg Config, maxProbeDepthError float64) (ExperimentReport, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ExperimentReport{}, err
	}
	scene := NewScene(cfg)
	decision := DecideOrder(scene, cfg, maxProbeDepthError)
	low := RunLowResDistortThenReproject(scene, cfg)
	wrong := RunWrongDepthReuse(scene, cfg)
	channels := RunPerChannelDistortion(scene, cfg)
	report := BuildExperimentReport(cfg, maxProbeDepthError)

	files := []struct {
		name   string
		raster *Raster
	}{
		{"source.png", scene.Raster},
		{"reference.png", decision.Reference},
		{"reproject-then-distort.png", decision.A.Result.Final},
		{"distort-then-reproject.png", decision.B.Result.Final},
		{"lowres-distort-then-reproject.png", low.Final},
		{"wrong-depth.png", wrong.Final},
	}
	for _, f := range files {
		if err := writeRasterPNG(filepath.Join(dir, f.name), f.raster); err != nil {
			return report, err
		}
	}
	if err := writePerChannelPNG(filepath.Join(dir, "per-channel.png"), channels); err != nil {
		return report, err
	}
	for _, f := range []struct {
		name   string
		raster *Raster
	}{
		{"depth-reference.png", decision.Reference},
		{"depth-reproject-then-distort.png", decision.A.Result.Final},
		{"depth-distort-then-reproject.png", decision.B.Result.Final},
	} {
		if err := writeDepthPNG(filepath.Join(dir, f.name), f.raster); err != nil {
			return report, err
		}
	}
	jsonData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return report, err
	}
	jsonData = append(jsonData, '\n')
	if err := os.WriteFile(filepath.Join(dir, "report.json"), jsonData, 0o644); err != nil {
		return report, err
	}
	if err := os.WriteFile(filepath.Join(dir, "report.md"), []byte(ReportMarkdown(report)), 0o644); err != nil {
		return report, err
	}
	return report, nil
}

func writeRasterPNG(path string, r *Raster) error {
	img := image.NewRGBA(image.Rect(0, 0, r.W, r.H))
	for j := 0; j < r.H; j++ {
		for i := 0; i < r.W; i++ {
			n := j*r.W + i
			if !r.Filled[n] {
				continue
			}
			c := r.Color[n]
			off := img.PixOffset(i, j)
			img.Pix[off] = c.R
			img.Pix[off+1] = c.G
			img.Pix[off+2] = c.B
			img.Pix[off+3] = 255
		}
	}
	return writePNG(path, img)
}

func writeDepthPNG(path string, r *Raster) error {
	const minDepth, maxDepth = 0.4, 2.6
	img := image.NewRGBA(image.Rect(0, 0, r.W, r.H))
	for j := 0; j < r.H; j++ {
		for i := 0; i < r.W; i++ {
			n := j*r.W + i
			if !r.Filled[n] {
				continue
			}
			v := (r.Depth[n] - minDepth) / (maxDepth - minDepth)
			if v < 0 {
				v = 0
			} else if v > 1 {
				v = 1
			}
			g := uint8(255 * v)
			off := img.PixOffset(i, j)
			img.Pix[off], img.Pix[off+1], img.Pix[off+2], img.Pix[off+3] = g, g, g, 255
		}
	}
	return writePNG(path, img)
}

func writePerChannelPNG(path string, cr ChannelResult) error {
	w, h := cr.Channels[0].W, cr.Channels[0].H
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for j := 0; j < h; j++ {
		for i := 0; i < w; i++ {
			n := j*w + i
			var channels [3]uint8
			for c := 0; c < 3; c++ {
				if cr.Channels[c].Filled[n] {
					channels[c] = cr.Channels[c].Color[n].R
				}
			}
			off := img.PixOffset(i, j)
			img.Pix[off], img.Pix[off+1], img.Pix[off+2], img.Pix[off+3] =
				channels[0], channels[1], channels[2], 255
		}
	}
	return writePNG(path, img)
}

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		_ = f.Close()
		return fmt.Errorf("encode %s: %w", path, err)
	}
	return f.Close()
}
