package eyebuffer

import (
	"image"
	"strconv"
	"testing"
)

func mustGenerate(t *testing.T, eyes []EyeSpec, opts LayoutOptions) *Atlas {
	t.Helper()
	atlas, err := Generate(eyes, opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	return atlas
}

func requireClean(t *testing.T, atlas *Atlas, opts ValidationOptions) {
	t.Helper()
	report := Validate(atlas, opts)
	if !report.OK() {
		t.Fatalf("validation failed:\n%s", issueText(report))
	}
	if report.Samples == 0 {
		t.Fatal("validation sampled nothing")
	}
	if report.Texels == 0 {
		t.Fatal("validation scanned no texels")
	}
}

func hasIssueKind(report Report, kind IssueKind) bool {
	for _, issue := range report.Issues {
		if issue.Kind == kind {
			return true
		}
	}
	return false
}

func issueText(report Report) string {
	text := ""
	for _, issue := range report.Issues {
		text += issue.String() + "\n"
	}
	if report.Suppressed > 0 {
		text += "suppressed additional issues: " + strconv.Itoa(report.Suppressed)
	}
	return text
}

func TestUnequalEyesGetIndependentViewports(t *testing.T) {
	eyes := []EyeSpec{
		{Name: "left", Width: 257, Height: 129, Mipmaps: true},
		{Name: "right", Width: 191, Height: 97, Mipmaps: false},
	}
	atlas := mustGenerate(t, eyes, LayoutOptions{Gutter: 3, Alignment: 4})
	layout := atlas.Layout
	if len(layout.Eyes) != 2 {
		t.Fatalf("got %d eyes, want 2", len(layout.Eyes))
	}
	left, right := layout.Eyes[0], layout.Eyes[1]
	if left.PixelWidth == right.PixelWidth || left.PixelHeight == right.PixelHeight {
		t.Fatalf("test needs unequal eye dimensions, got %dx%d and %dx%d",
			left.PixelWidth, left.PixelHeight, right.PixelWidth, right.PixelHeight)
	}
	if !left.Slot.Intersect(right.Slot).Empty() {
		t.Fatalf("eye slots overlap: %s and %s", left.Slot, right.Slot)
	}
	if len(left.Levels) <= 1 {
		t.Fatalf("left eye should have mips, got %d levels", len(left.Levels))
	}
	if len(right.Levels) != 1 {
		t.Fatalf("right eye mips are disabled, got %d levels", len(right.Levels))
	}
	for _, eye := range layout.Eyes {
		for i, lv := range eye.Levels {
			if !lv.Rect.In(eye.Slot) {
				t.Errorf("%s level %d %s is outside slot %s", eye.Name, i, lv.Rect, eye.Slot)
			}
			if lv.PackedStride%4 != 0 {
				t.Errorf("%s level %d stride %d is not 4-byte aligned", eye.Name, i, lv.PackedStride)
			}
		}
	}
	if layout.Stride%layout.Alignment != 0 {
		t.Fatalf("atlas stride %d is not aligned to %d", layout.Stride, layout.Alignment)
	}
	requireClean(t, atlas, DefaultValidationOptions())
}

func TestSingleEyeMipDisabledAndPolicyMismatch(t *testing.T) {
	eyes := []EyeSpec{
		{Name: "left", Width: 64, Height: 48, Mipmaps: true},
		{Name: "right", Width: 37, Height: 29, Mipmaps: false},
	}
	atlas := mustGenerate(t, eyes, LayoutOptions{Gutter: 2})
	right := atlas.Layout.Eyes[1]
	if len(right.Levels) != 1 {
		t.Fatalf("right eye has %d levels, want base only", len(right.Levels))
	}
	if right.Levels[0].Rect != image.Rect(right.Slot.Min.X+2, 2,
		right.Slot.Min.X+2+37, 2+29) {
		t.Fatalf("unexpected right base rect %s", right.Levels[0].Rect)
	}
	requireClean(t, atlas, DefaultValidationOptions())

	atlas.Layout.Eyes[1].MipmapsEnabled = true
	report := Validate(atlas, DefaultValidationOptions())
	if !hasIssueKind(report, IssueMipPolicy) {
		t.Fatalf("disabled mip mismatch not reported:\n%s", issueText(report))
	}
}

func TestHighDPIScalesBothEyesAndKeepsAlignment(t *testing.T) {
	eyes := []EyeSpec{
		{Name: "left", Width: 101, Height: 67, Mipmaps: true},
		{Name: "right", Width: 53, Height: 31, Mipmaps: true},
	}
	atlas := mustGenerate(t, eyes, LayoutOptions{Gutter: 2, Alignment: 4, Scale: 2})
	left, right := atlas.Layout.Eyes[0], atlas.Layout.Eyes[1]
	if left.LogicalWidth != 101 || left.LogicalHeight != 67 {
		t.Fatalf("logical size changed under high DPI: %dx%d", left.LogicalWidth, left.LogicalHeight)
	}
	if left.PixelWidth != 202 || left.PixelHeight != 134 {
		t.Fatalf("left high-DPI size = %dx%d, want 202x134", left.PixelWidth, left.PixelHeight)
	}
	if right.PixelWidth != 106 || right.PixelHeight != 62 {
		t.Fatalf("right high-DPI size = %dx%d, want 106x62", right.PixelWidth, right.PixelHeight)
	}
	if atlas.Layout.Scale != 2 {
		t.Fatalf("layout scale = %v, want 2", atlas.Layout.Scale)
	}
	requireClean(t, atlas, DefaultValidationOptions())
}

func TestZeroGutterIsReported(t *testing.T) {
	eyes := []EyeSpec{
		{Name: "left", Width: 32, Height: 24, Mipmaps: true},
		{Name: "right", Width: 24, Height: 16, Mipmaps: true},
	}
	atlas := mustGenerate(t, eyes, LayoutOptions{Gutter: 0})
	report := Validate(atlas, DefaultValidationOptions())
	if !hasIssueKind(report, IssueGutter) {
		t.Fatalf("zero gutter was not reported:\n%s", issueText(report))
	}
}

func TestBuildRejectsNonFourByteAlignment(t *testing.T) {
	_, err := BuildLayout([]EyeSpec{{Name: "eye", Width: 7, Height: 5}}, LayoutOptions{Alignment: 2})
	if err == nil {
		t.Fatal("2-byte alignment was accepted for RGBA8 rows")
	}
}
