package render

import (
	"image"
	"testing"

	"stereogaze/internal/gaze"
)

func TestRenderFrameProducesFivePanels(t *testing.T) {
	left, err := gaze.BuildMask(gaze.EyeConfig{
		Eye: gaze.LeftEye, Gaze: gaze.Point{X: 0.35, Y: 0.5}, Params: gaze.DefaultParams(),
	}, 24)
	if err != nil {
		t.Fatalf("left BuildMask: %v", err)
	}
	right, err := gaze.BuildMask(gaze.EyeConfig{
		Eye: gaze.RightEye, Gaze: gaze.Point{X: 0.65, Y: 0.5}, Params: gaze.DefaultParams(),
	}, 24)
	if err != nil {
		t.Fatalf("right BuildMask: %v", err)
	}
	report, err := gaze.AnalyzeSharing(left, right)
	if err != nil {
		t.Fatalf("AnalyzeSharing: %v", err)
	}

	frame := RenderFrame(left, right, report, 3)
	wantWidth := 5*panelSize + 4*panelGap
	bounds := frame.Bounds()
	if bounds.Dx() != wantWidth || bounds.Dy() != panelSize {
		t.Fatalf("frame size = %dx%d, want %dx%d", bounds.Dx(), bounds.Dy(), wantWidth, panelSize)
	}

	var animated = AnimatedGIF([]*image.Paletted{frame, frame}, 8)
	if len(animated.Image) != 2 || animated.LoopCount != 0 {
		t.Fatal("animated GIF did not retain both frames or loop config")
	}
	if animated.Delay[0] != 8 || animated.Delay[1] != 8 {
		t.Fatalf("GIF delays = %v, want 8", animated.Delay)
	}
}
