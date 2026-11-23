package eyebuffer

import (
	"image/color"
	"testing"
)

func TestEdgeSamplingNeverLeavesMipLevel(t *testing.T) {
	eyes := []EyeSpec{
		{Name: "left", Width: 33, Height: 21, Mipmaps: true},
		{Name: "right", Width: 19, Height: 13, Mipmaps: true},
	}
	atlas := mustGenerate(t, eyes, LayoutOptions{Gutter: 2})
	uvs := []float64{0, 0.000001, 0.25, 0.5, 0.75, 0.999999, 1}
	for eyeIndex, eye := range atlas.Layout.Eyes {
		for level, lv := range eye.Levels {
			for _, filter := range []Filter{FilterNearest, FilterBilinear} {
				for _, u := range uvs {
					for _, v := range uvs {
						sample, err := atlas.Sample(eyeIndex, level, u, v, filter)
						if err != nil {
							t.Fatalf("%s level %d uv=(%v,%v): %v", eye.Name, level, u, v, err)
						}
						for _, tap := range sample.Taps {
							if !tap.Atlas.In(lv.Rect) {
								t.Fatalf("%s level %d uv=(%v,%v) filter=%s tap %s left %s",
									eye.Name, level, u, v, filter, tap.Atlas, lv.Rect)
							}
						}
						want := ExpectedSample(eyeIndex, level, lv.Width, lv.Height, u, v, filter)
						if sample.Color != want {
							t.Fatalf("%s level %d uv=(%v,%v) filter=%s: got %s want %s",
								eye.Name, level, u, v, filter, formatRGBA(sample.Color), formatRGBA(want))
						}
					}
				}
			}
		}
	}
}

func TestAtlasClampCrossesIntoNeighbouringEye(t *testing.T) {
	eyes := []EyeSpec{
		{Name: "left", Width: 32, Height: 24},
		{Name: "right", Width: 24, Height: 16},
	}
	atlas := mustGenerate(t, eyes, LayoutOptions{Gutter: 0})
	opts := ValidationOptions{
		Filter:          FilterNearest,
		EdgeSamples:     5,
		InteriorSamples: 2,
		Address:         ClampToAtlas,
	}
	report := Validate(atlas, opts)
	if !hasIssueKind(report, IssueCrossRegion) {
		t.Fatalf("atlas-wide clamp was not diagnosed as cross-region:\n%s", issueText(report))
	}
}

func TestAtlasClampBleedsGutterSentinelAtEdge(t *testing.T) {
	eyes := []EyeSpec{
		{Name: "left", Width: 16, Height: 16},
		{Name: "right", Width: 16, Height: 16},
	}
	atlas := mustGenerate(t, eyes, LayoutOptions{Gutter: 2})
	sample, err := atlas.SampleWithAddress(1, 0, 0, 0, FilterBilinear, ClampToAtlas)
	if err != nil {
		t.Fatalf("SampleWithAddress: %v", err)
	}
	foundGutterTap := false
	for _, tap := range sample.Taps {
		if c, ok := atlas.Pixel(tap.Atlas.X, tap.Atlas.Y); ok && c == GutterColor {
			foundGutterTap = true
		}
	}
	if !foundGutterTap {
		t.Fatalf("edge sample did not fetch any gutter sentinel: %+v", sample.Taps)
	}
	want := ExpectedSample(1, 0, 16, 16, 0, 0, FilterBilinear)
	if sample.Color == want {
		t.Fatalf("buggy atlas clamp unexpectedly matched level clamp: %s", formatRGBA(want))
	}
	opts := DefaultValidationOptions()
	opts.Address = ClampToAtlas
	report := Validate(atlas, opts)
	if !hasIssueKind(report, IssueGutter) {
		t.Fatalf("gutter bleed was not reported:\n%s", issueText(report))
	}
}

func TestStrideMetadataAndMisalignmentAreReported(t *testing.T) {
	eyes := []EyeSpec{
		{Name: "left", Width: 257, Height: 65, Mipmaps: true},
		{Name: "right", Width: 191, Height: 49, Mipmaps: true},
	}
	atlas := mustGenerate(t, eyes, LayoutOptions{Gutter: 2})
	goodStride := atlas.Layout.Stride
	atlas.Layout.Stride = goodStride - 1
	report := Validate(atlas, DefaultValidationOptions())
	if !hasIssueKind(report, IssueStride) {
		t.Fatalf("wrong stride was not reported:\n%s", issueText(report))
	}
	if !hasIssueKind(report, IssueAlignment) {
		t.Fatalf("misaligned stride was not reported:\n%s", issueText(report))
	}
	for _, issue := range report.Issues {
		if issue.Kind == IssueStride && issue.Expected != "" && issue.Actual != "" {
			return
		}
	}
	t.Fatalf("stride issue lacks expected/actual values:\n%s", issueText(report))
}

func TestCorruptedTexelIsReportedAsColorBleed(t *testing.T) {
	eyes := []EyeSpec{
		{Name: "left", Width: 32, Height: 24},
		{Name: "right", Width: 24, Height: 16},
	}
	atlas := mustGenerate(t, eyes, LayoutOptions{Gutter: 2})
	left := atlas.Layout.Eyes[0].Levels[0]
	atlas.SetPixel(left.Rect.Min.X+16, left.Rect.Min.Y+12, GutterColor)
	opts := ValidationOptions{
		Filter:          FilterNearest,
		EdgeSamples:     3,
		InteriorSamples: 3,
		Address:         ClampToLevel,
	}
	report := Validate(atlas, opts)
	if !hasIssueKind(report, IssueColorBleed) {
		t.Fatalf("corrupted texel was not reported as colour bleed:\n%s", issueText(report))
	}
}

func TestTexelScanCatchesPixelOutsideSampleGrid(t *testing.T) {
	eyes := []EyeSpec{{Name: "eye", Width: 64, Height: 64}}
	atlas := mustGenerate(t, eyes, LayoutOptions{Gutter: 2})
	lv := atlas.Layout.Eyes[0].Levels[0]
	atlas.SetPixel(lv.Rect.Min.X+1, lv.Rect.Min.Y+1, GutterColor)
	opts := ValidationOptions{
		Filter:          FilterNearest,
		EdgeSamples:     2,
		InteriorSamples: 1,
		Address:         ClampToLevel,
	}
	report := Validate(atlas, opts)
	if !hasIssueKind(report, IssueColorBleed) {
		t.Fatalf("off-grid corrupt texel was not reported:\n%s", issueText(report))
	}
	for _, issue := range report.Issues {
		if issue.Kind == IssueColorBleed && issue.Where == "local texel (1,1) at atlas (3,3)" {
			return
		}
	}
	t.Fatalf("diagnostic did not point at local texel (1,1):\n%s", issueText(report))
}

func TestTexelFromOtherEyeIsClassifiedAsCrossRegion(t *testing.T) {
	eyes := []EyeSpec{
		{Name: "left", Width: 32, Height: 24},
		{Name: "right", Width: 24, Height: 16},
	}
	atlas := mustGenerate(t, eyes, LayoutOptions{Gutter: 2})
	left := atlas.Layout.Eyes[0].Levels[0]
	atlas.SetPixel(left.Rect.Min.X+16, left.Rect.Min.Y+12,
		TexelColor(1, 0, 16, 12, 24, 16))
	opts := ValidationOptions{
		Filter:          FilterNearest,
		EdgeSamples:     3,
		InteriorSamples: 3,
		Address:         ClampToLevel,
	}
	report := Validate(atlas, opts)
	if !hasIssueKind(report, IssueCrossRegion) {
		t.Fatalf("foreign-eye texel was not classified as cross-region:\n%s", issueText(report))
	}
}

func TestCopyLevelValidatesSourceStride(t *testing.T) {
	atlas := mustGenerate(t, []EyeSpec{{Name: "eye", Width: 3, Height: 2}}, LayoutOptions{Gutter: 1})
	lv := atlas.Layout.Eyes[0].Levels[0]

	if err := atlas.CopyLevel(0, 0, make([]byte, 8), 8); err == nil {
		t.Fatal("stride smaller than width span was accepted")
	}
	if err := atlas.CopyLevel(0, 0, make([]byte, 13*2), 13); err == nil {
		t.Fatal("non-4-byte-aligned stride was accepted")
	}

	srcStride := 16
	src := make([]byte, srcStride*2)
	for y := 0; y < 2; y++ {
		for x := 0; x < 3; x++ {
			off := y*srcStride + x*4
			src[off] = 10 + byte(x)
			src[off+1] = 20 + byte(y)
			src[off+2] = 30
			src[off+3] = 255
		}
	}
	if err := atlas.CopyLevel(0, 0, src, srcStride); err != nil {
		t.Fatalf("valid padded source rejected: %v", err)
	}
	got, ok := atlas.Pixel(lv.Rect.Min.X+2, lv.Rect.Min.Y+1)
	want := color.RGBA{R: 12, G: 21, B: 30, A: 255}
	if !ok || got != want {
		t.Fatalf("copied pixel = %s ok=%t, want %s", formatRGBA(got), ok, formatRGBA(want))
	}
	if !lv.Rect.In(atlas.Layout.Eyes[0].Slot) {
		t.Fatalf("test level escaped slot")
	}
}

func TestValidationCanUseSourceTexelFunction(t *testing.T) {
	atlas := mustGenerate(t, []EyeSpec{{Name: "eye", Width: 5, Height: 4}}, LayoutOptions{Gutter: 2})
	level := atlas.Layout.Eyes[0].Levels[0]
	srcStride := 24
	src := make([]byte, srcStride*level.Height)
	expected := func(eyeIndex, level, x, y, width, height int) color.RGBA {
		return color.RGBA{R: uint8(20 + x), G: uint8(40 + y), B: 90, A: 255}
	}
	for y := 0; y < level.Height; y++ {
		for x := 0; x < level.Width; x++ {
			c := expected(0, 0, x, y, level.Width, level.Height)
			off := y*srcStride + x*4
			src[off], src[off+1], src[off+2], src[off+3] = c.R, c.G, c.B, c.A
		}
	}
	if err := atlas.CopyLevel(0, 0, src, srcStride); err != nil {
		t.Fatalf("CopyLevel: %v", err)
	}
	opts := DefaultValidationOptions()
	opts.Expected = expected
	requireClean(t, atlas, opts)
}
