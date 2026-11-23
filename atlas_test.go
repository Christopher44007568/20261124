package eyebuffer

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestWritePNGPreservesVisiblePixels(t *testing.T) {
	eyes := []EyeSpec{
		{Name: "left", Width: 31, Height: 19, Mipmaps: true},
		{Name: "right", Width: 17, Height: 11},
	}
	atlas := mustGenerate(t, eyes, LayoutOptions{Gutter: 2, Alignment: 8})
	path := filepath.Join(t.TempDir(), "atlas.png")
	if err := atlas.WritePNG(path); err != nil {
		t.Fatalf("WritePNG: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open PNG: %v", err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode PNG: %v", err)
	}
	if got := img.Bounds(); got.Dx() != atlas.Layout.Width || got.Dy() != atlas.Layout.Height {
		t.Fatalf("PNG bounds = %v, want %dx%d", got, atlas.Layout.Width, atlas.Layout.Height)
	}
	lv := atlas.Layout.Eyes[0].Levels[0]
	want, _ := atlas.Pixel(lv.Rect.Min.X+7, lv.Rect.Min.Y+5)
	r, g, b, a := img.At(lv.Rect.Min.X+7, lv.Rect.Min.Y+5).RGBA()
	got := colorRGBA8(r, g, b, a)
	if got != want {
		t.Fatalf("PNG pixel = %s, atlas pixel = %s", formatRGBA(got), formatRGBA(want))
	}
}

func colorRGBA8(r, g, b, a uint32) color.RGBA {
	return color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
}

func TestLevelOverlapIsReported(t *testing.T) {
	eyes := []EyeSpec{
		{Name: "left", Width: 16, Height: 16},
		{Name: "right", Width: 16, Height: 16},
	}
	atlas := mustGenerate(t, eyes, LayoutOptions{Gutter: 2})
	left := &atlas.Layout.Eyes[0].Levels[0]
	right := &atlas.Layout.Eyes[1].Levels[0]
	right.Rect = image.Rect(
		left.Rect.Max.X-1,
		left.Rect.Min.Y,
		left.Rect.Max.X-1+right.Width,
		left.Rect.Min.Y+right.Height,
	)
	report := Validate(atlas, DefaultValidationOptions())
	if !hasIssueKind(report, IssueOverlap) {
		t.Fatalf("overlapping levels were not reported:\n%s", issueText(report))
	}
}
