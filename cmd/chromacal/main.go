package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"

	"github.com/Christopher44007568/20261124/chart"
	"github.com/Christopher44007568/20261124/chroma"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "chromacal:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		width     = flag.Int("width", 960, "output width in pixels")
		height    = flag.Int("height", 720, "output height in pixels")
		outDir    = flag.String("out", "out", "output directory")
		eyeName   = flag.String("eye", "left", "calibration eye: left or right")
		edgeName  = flag.String("edge", "black", "out-of-bounds mode: black, clamp, transparent")
		red       = flag.Float64("red", 0.012, "base red radial coefficient")
		green     = flag.Float64("green", 0, "base green radial coefficient")
		blue      = flag.Float64("blue", -0.012, "base blue radial coefficient")
		biasRed   = flag.Float64("bias-red", 0, "selected-eye red calibration bias")
		biasGreen = flag.Float64("bias-green", 0, "selected-eye green calibration bias")
		biasBlue  = flag.Float64("bias-blue", 0, "selected-eye blue calibration bias")
		centerX   = flag.Float64("center-x", 0, "selected-eye optical center X offset (normalized)")
		centerY   = flag.Float64("center-y", 0, "selected-eye optical center Y offset (normalized)")
	)
	flag.Parse()

	eye, err := parseEye(*eyeName)
	if err != nil {
		return err
	}
	edge, err := parseEdge(*edgeName)
	if err != nil {
		return err
	}
	cfg := chroma.Config{
		Aberration: chroma.Aberration{Red: *red, Green: *green, Blue: *blue},
		Eye:        eye,
		Edge:       edge,
	}
	calibration := chroma.EyeCalibration{
		CenterOffset: chroma.Point{X: *centerX, Y: *centerY},
		RadialBias:   chroma.Aberration{Red: *biasRed, Green: *biasGreen, Blue: *biasBlue},
	}
	if eye == chroma.RightEye {
		cfg.Calibration.Right = calibration
	} else {
		cfg.Calibration.Left = calibration
	}

	source, err := chart.RGBTestChart(*width, *height)
	if err != nil {
		return err
	}
	result, err := chroma.Warp(source, nil, cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		return err
	}

	files := []struct {
		name string
		img  image.Image
	}{
		{"source.png", source},
		{"warped.png", result.Image},
		{"channel-red.png", channelImage(result.Image, chroma.Red)},
		{"channel-green.png", channelImage(result.Image, chroma.Green)},
		{"channel-blue.png", channelImage(result.Image, chroma.Blue)},
	}
	for _, file := range files {
		if err := writePNG(filepath.Join(*outDir, file.name), file.img); err != nil {
			return err
		}
	}

	fmt.Printf("eye=%s edge=%s size=%dx%d\n", eye, edge, *width, *height)
	fmt.Printf("effective coefficients: R=%g G=%g B=%g\n",
		cfg.EffectiveAberration().Red,
		cfg.EffectiveAberration().Green,
		cfg.EffectiveAberration().Blue,
	)
	for ch := chroma.Red; ch <= chroma.Blue; ch++ {
		fmt.Printf("%-5s corner scale=%.6f\n", ch, chroma.RadialScale(cfg, chroma.Point{X: 1, Y: 1}, ch))
	}
	fmt.Printf("wrote %d PNG files to %s\n", len(files), *outDir)
	return nil
}

func parseEye(value string) (chroma.Eye, error) {
	switch value {
	case "left":
		return chroma.LeftEye, nil
	case "right":
		return chroma.RightEye, nil
	default:
		return 0, fmt.Errorf("unknown eye %q", value)
	}
}

func parseEdge(value string) (chroma.EdgeMode, error) {
	switch value {
	case "black":
		return chroma.EdgeBlack, nil
	case "clamp":
		return chroma.EdgeClamp, nil
	case "transparent":
		return chroma.EdgeTransparent, nil
	default:
		return 0, fmt.Errorf("unknown edge mode %q", value)
	}
}

func channelImage(src *image.RGBA, ch chroma.Channel) *image.Gray {
	out := image.NewGray(src.Bounds())
	for y := src.Bounds().Min.Y; y < src.Bounds().Max.Y; y++ {
		for x := src.Bounds().Min.X; x < src.Bounds().Max.X; x++ {
			offset := src.PixOffset(x, y)
			value := src.Pix[offset]
			if ch == chroma.Green {
				value = src.Pix[offset+1]
			}
			if ch == chroma.Blue {
				value = src.Pix[offset+2]
			}
			out.SetGray(x, y, color.Gray{Y: value})
		}
	}
	return out
}

func writePNG(path string, img image.Image) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return png.Encode(file, img)
}
