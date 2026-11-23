package main

import (
	"flag"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"eyebuffer"
)

func main() {
	leftSize := flag.String("left", "257x129", "left eye logical size, WxH")
	rightSize := flag.String("right", "191x97", "right eye logical size, WxH")
	leftMips := flag.Bool("left-mips", true, "enable mipmaps for the left eye")
	rightMips := flag.Bool("right-mips", false, "enable mipmaps for the right eye")
	gutter := flag.Int("gutter", 3, "gutter in pixels around each base level and mip")
	alignment := flag.Int("align", 4, "row-stride alignment in bytes")
	dpi := flag.Float64("dpi", 1, "render scale for high-DPI viewports")
	out := flag.String("out", "artifacts/eye-atlas.png", "atlas PNG output path")
	filterName := flag.String("filter", "bilinear", "sampling filter: nearest or bilinear")
	addressName := flag.String("address", "level", "address mode: level (correct) or atlas (fault probe)")
	fault := flag.String("fault", "none", "inject a demo fault: none, stride, bleed, overlap, gutter, cross")
	flag.Parse()

	leftW, leftH, err := parseSize(*leftSize)
	if err != nil {
		fail("-left: %v", err)
	}
	rightW, rightH, err := parseSize(*rightSize)
	if err != nil {
		fail("-right: %v", err)
	}
	filter, err := parseFilter(*filterName)
	if err != nil {
		fail("-filter: %v", err)
	}
	address, err := parseAddress(*addressName)
	if err != nil {
		fail("-address: %v", err)
	}

	opts := eyebuffer.LayoutOptions{Gutter: *gutter, Alignment: *alignment, Scale: *dpi}
	if strings.EqualFold(*fault, "gutter") || strings.EqualFold(*fault, "cross") {
		opts.Gutter = 0
	}
	eyes := []eyebuffer.EyeSpec{
		{Name: "left", Width: leftW, Height: leftH, Mipmaps: *leftMips},
		{Name: "right", Width: rightW, Height: rightH, Mipmaps: *rightMips},
	}
	atlas, err := eyebuffer.Generate(eyes, opts)
	if err != nil {
		fail("generate: %v", err)
	}
	if err := applyFault(atlas, strings.ToLower(*fault)); err != nil {
		fail("-fault: %v", err)
	}
	if strings.EqualFold(*fault, "cross") {
		address = eyebuffer.ClampToAtlas
	}

	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fail("create output directory: %v", err)
	}
	if err := atlas.WritePNG(*out); err != nil {
		fail("write PNG: %v", err)
	}

	report := eyebuffer.Validate(atlas, eyebuffer.ValidationOptions{
		Filter:          filter,
		EdgeSamples:     9,
		InteriorSamples: 5,
		Address:         address,
	})
	printReport(report, atlas, *out)
	if !report.OK() {
		os.Exit(1)
	}
}

func parseSize(value string) (int, int, error) {
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == 'x' || r == 'X' || r == '*' })
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("want WxH, got %q", value)
	}
	w, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid width %q", parts[0])
	}
	h, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid height %q", parts[1])
	}
	return w, h, nil
}

func parseFilter(value string) (eyebuffer.Filter, error) {
	switch strings.ToLower(value) {
	case "nearest", "point":
		return eyebuffer.FilterNearest, nil
	case "bilinear", "linear":
		return eyebuffer.FilterBilinear, nil
	default:
		return 0, fmt.Errorf("want nearest or bilinear, got %q", value)
	}
}

func parseAddress(value string) (eyebuffer.AddressMode, error) {
	switch strings.ToLower(value) {
	case "level", "viewport":
		return eyebuffer.ClampToLevel, nil
	case "atlas":
		return eyebuffer.ClampToAtlas, nil
	default:
		return 0, fmt.Errorf("want level or atlas, got %q", value)
	}
}

func applyFault(atlas *eyebuffer.Atlas, fault string) error {
	switch fault {
	case "", "none":
		return nil
	case "stride":
		atlas.Layout.Stride--
	case "bleed":
		if len(atlas.Layout.Eyes) < 2 || len(atlas.Layout.Eyes[0].Levels) == 0 {
			return fmt.Errorf("bleed probe needs two eyes")
		}
		left := atlas.Layout.Eyes[0].Levels[0]
		x, y := left.Width/2, left.Height/2
		atlas.SetPixel(left.Rect.Min.X+x, left.Rect.Min.Y+y,
			eyebuffer.TexelColor(1, 0, x, y, left.Width, left.Height))
	case "overlap":
		if len(atlas.Layout.Eyes) < 2 {
			return fmt.Errorf("overlap probe needs two eyes")
		}
		left := atlas.Layout.Eyes[0].Levels[0]
		right := &atlas.Layout.Eyes[1].Levels[0]
		right.Rect = image.Rect(left.Rect.Max.X-1, left.Rect.Min.Y,
			left.Rect.Max.X-1+right.Width, left.Rect.Min.Y+right.Height)
	case "gutter":
		// The layout was generated with Gutter=0 above.
	case "cross":
		// The address mode is switched to atlas-wide clamping.
	default:
		return fmt.Errorf("unknown fault %q", fault)
	}
	return nil
}

func printReport(report eyebuffer.Report, atlas *eyebuffer.Atlas, out string) {
	status := "PASS"
	if !report.OK() {
		status = "FAIL"
	}
	fmt.Printf("%s atlas=%dx%d stride=%d alignment=%d gutter=%d scale=%g\n",
		status, report.Width, report.Height, report.Stride,
		atlas.Layout.Alignment, atlas.Layout.Gutter, atlas.Layout.Scale)
	fmt.Printf("file: %s\n", out)
	for _, eye := range atlas.Layout.Eyes {
		fmt.Printf("  %-5s logical=%dx%d physical=%dx%d levels=%d base-row=%d slot=%s\n",
			eye.Name, eye.LogicalWidth, eye.LogicalHeight, eye.PixelWidth, eye.PixelHeight,
			len(eye.Levels), eye.BaseRowStride, eye.Slot)
	}
	fmt.Printf("texels=%d samples=%d issues=%d\n",
		report.Texels, report.Samples, len(report.Issues)+report.Suppressed)
	for _, issue := range report.Issues {
		fmt.Printf("  %s\n", issue.String())
	}
	if report.Suppressed > 0 {
		fmt.Printf("  ... %d additional issues suppressed\n", report.Suppressed)
	}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "eyebuffer-verify: "+format+"\n", args...)
	os.Exit(2)
}
