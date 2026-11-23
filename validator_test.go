package reproject

import (
	"math"
	"testing"
)

const (
	testWidth  = 8
	testHeight = 8
	bgDepth    = 5
)

func testCamera() Camera {
	return Camera{
		Right:   Vec3{X: 1},
		Up:      Vec3{Y: 1},
		Forward: Vec3{Z: 1},
		FX:      1,
		FY:      1,
	}
}

func facingNormal() Vec3 {
	return Vec3{Z: -1}
}

func makeFrame(depthAt func(x, y int) float32, confidenceAt func(x, y int) float32, normalAt func(x, y int) Vec3) Frame {
	frame := Frame{
		Width:      testWidth,
		Height:     testHeight,
		Depth:      make([]float32, testWidth*testHeight),
		Normals:    make([]Vec3, testWidth*testHeight),
		Confidence: make([]float32, testWidth*testHeight),
	}
	for y := 0; y < testHeight; y++ {
		for x := 0; x < testWidth; x++ {
			index := frame.index(x, y)
			frame.Depth[index] = depthAt(x, y)
			frame.Confidence[index] = confidenceAt(x, y)
			frame.Normals[index] = normalAt(x, y)
		}
	}
	return frame
}

func uniformScene() Frame {
	return makeFrame(
		func(int, int) float32 { return bgDepth },
		func(int, int) float32 { return 1 },
		func(int, int) Vec3 { return facingNormal() },
	)
}

func resultAt(t *testing.T, mask Mask, x, y int) PixelResult {
	t.Helper()
	if x < 0 || y < 0 || x >= mask.Width || y >= mask.Height {
		t.Fatalf("pixel (%d,%d) outside mask", x, y)
	}
	return mask.Pixels[y*mask.Width+x]
}

func assertRejected(t *testing.T, result PixelResult, want RejectReason) {
	t.Helper()
	if result.Valid {
		t.Fatalf("pixel was accepted: %+v", result)
	}
	if result.Reason != want {
		t.Fatalf("reason = %d, want %d", result.Reason, want)
	}
}

func assertConservativeMask(t *testing.T, mask Mask, stats Stats) {
	t.Helper()
	if stats.RejectionFraction >= 1 || stats.Accepted == 0 {
		t.Fatalf("mask discarded the whole screen: %+v", stats)
	}
	for _, pixel := range mask.Pixels {
		if pixel.Valid != (pixel.Reason == RejectNone) {
			t.Fatalf("valid flag and reason disagree: %+v", pixel)
		}
	}
}

func TestNearWallThinPoleAndTransparentEdgeAreExplicit(t *testing.T) {
	validator, err := NewValidator(DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}

	current := makeFrame(
		func(x, y int) float32 {
			if x == 0 && y == 0 {
				return 0
			}
			if x == 3 && (y == 3 || y == 4) {
				return 4
			}
			return bgDepth
		},
		func(x, y int) float32 {
			if x == 0 && y == 0 {
				return 0
			}
			return 1
		},
		func(int, int) Vec3 { return facingNormal() },
	)
	history := makeFrame(
		func(x, y int) float32 {
			if x == 4 && (y == 3 || y == 4) {
				return 4
			}
			if x == 0 && y == 1 {
				return 0
			}
			return bgDepth
		},
		func(x, y int) float32 {
			if x == 0 && y == 1 {
				return 0.05
			}
			return 1
		},
		func(int, int) Vec3 { return facingNormal() },
	)

	mask, stats, err := validator.Evaluate(current, testCamera(), history, testCamera())
	if err != nil {
		t.Fatal(err)
	}
	assertConservativeMask(t, mask, stats)
	assertRejected(t, resultAt(t, mask, 0, 0), RejectCurrentInvalid)
	assertRejected(t, resultAt(t, mask, 0, 1), RejectLowHistoryConfidence)
	assertRejected(t, resultAt(t, mask, 3, 3), RejectDisocclusion)
	assertRejected(t, resultAt(t, mask, 4, 3), RejectDepthMismatch)
	if resultAt(t, mask, 6, 6).Rejected() {
		t.Fatal("stable background was rejected")
	}
	if stats.Disocclusion == 0 || stats.DepthMismatch == 0 || stats.LowHistory == 0 {
		t.Fatalf("expected explicit surface reject reasons, got %+v", stats)
	}
}

func TestForwardMotionRejectsDisocclusionButKeepsStableWall(t *testing.T) {
	validator, err := NewValidator(DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}

	current := makeFrame(
		func(x, y int) float32 {
			if x == 3 && (y == 3 || y == 4) {
				return 4
			}
			return bgDepth
		},
		func(int, int) float32 { return 1 },
		func(int, int) Vec3 { return facingNormal() },
	)
	history := makeFrame(
		func(int, int) float32 { return bgDepth + 0.2 },
		func(int, int) float32 { return 1 },
		func(int, int) Vec3 { return facingNormal() },
	)
	historyCamera := testCamera()
	historyCamera.Origin = Vec3{Z: -0.2}

	mask, stats, err := validator.Evaluate(current, testCamera(), history, historyCamera)
	if err != nil {
		t.Fatal(err)
	}
	assertConservativeMask(t, mask, stats)
	assertRejected(t, resultAt(t, mask, 3, 3), RejectDisocclusion)
	assertRejected(t, resultAt(t, mask, 3, 4), RejectDisocclusion)
	if resultAt(t, mask, 6, 6).Rejected() {
		t.Fatal("reprojected stable wall should remain valid under forward motion")
	}
	if stats.Accepted < stats.Total-stats.Rejected {
		t.Fatalf("accepted count is inconsistent: %+v", stats)
	}
}

func TestNonUnitFocalLengthRoundTripsStableSurface(t *testing.T) {
	validator, err := NewValidator(DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	camera := testCamera()
	camera.FX = 1.7
	camera.FY = 2.1

	mask, stats, err := validator.Evaluate(uniformScene(), camera, uniformScene(), camera)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Rejected != 0 {
		t.Fatalf("stable surface rejected with non-unit focal length: %+v", stats)
	}
	for _, pixel := range mask.Pixels {
		if !pixel.Valid {
			t.Fatalf("unexpected rejected pixel: %+v", pixel)
		}
	}
}

func rotatedCamera(angle float64) Camera {
	camera := testCamera()
	cos := float32(math.Cos(angle))
	sin := float32(math.Sin(angle))
	camera.Right = Vec3{X: cos, Z: -sin}
	camera.Forward = Vec3{X: sin, Z: cos}
	return camera
}

func TestRotationMarksOffscreenHistoryWithoutWholeScreenReject(t *testing.T) {
	limits := DefaultLimits()
	limits.DisocclusionSlope = 0.05
	limits.DepthRelativeSlope = 0.05
	validator, err := NewValidator(limits)
	if err != nil {
		t.Fatal(err)
	}

	current := uniformScene()
	historyCamera := rotatedCamera(math.Pi / 18)
	history := makeFrame(
		func(x, y int) float32 {
			worldPoint := unproject(x, y, testWidth, testHeight, bgDepth, historyCamera)
			if _, _, depth := project(worldPoint, testCamera(), testWidth, testHeight); depth <= 0 {
				return 0
			}
			return dot(sub(worldPoint, historyCamera.Origin), historyCamera.Forward)
		},
		func(x, y int) float32 {
			worldPoint := unproject(x, y, testWidth, testHeight, bgDepth, historyCamera)
			hx, hy, _ := project(worldPoint, testCamera(), testWidth, testHeight)
			if hx < 0 || hy < 0 {
				return 0
			}
			return 1
		},
		func(int, int) Vec3 {
			return worldNormalToView(facingNormal(), historyCamera)
		},
	)

	mask, stats, err := validator.Evaluate(current, testCamera(), history, historyCamera)
	if err != nil {
		t.Fatal(err)
	}
	assertConservativeMask(t, mask, stats)
	if stats.NoHistory == 0 {
		t.Fatalf("rotation did not mark pixels without history: %+v", stats)
	}
	if resultAt(t, mask, 3, 3).Rejected() {
		t.Fatalf("central overlapping surface should remain accepted: %+v", resultAt(t, mask, 3, 3))
	}
}

func TestObjectSwitchRejectsWrongForegroundAndChangedSurface(t *testing.T) {
	validator, err := NewValidator(DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}

	current := makeFrame(
		func(x, y int) float32 {
			if x == 2 && y == 2 {
				return 4.2
			}
			return bgDepth
		},
		func(int, int) float32 { return 1 },
		func(x, y int) Vec3 {
			if x == 5 && y == 5 {
				return Vec3{X: 1}
			}
			return facingNormal()
		},
	)
	history := makeFrame(
		func(x, y int) float32 {
			if x == 2 && y == 2 {
				return bgDepth
			}
			return bgDepth
		},
		func(int, int) float32 { return 1 },
		func(x, y int) Vec3 {
			if x == 5 && y == 5 {
				return facingNormal()
			}
			return facingNormal()
		},
	)

	mask, stats, err := validator.Evaluate(current, testCamera(), history, testCamera())
	if err != nil {
		t.Fatal(err)
	}
	assertConservativeMask(t, mask, stats)
	assertRejected(t, resultAt(t, mask, 2, 2), RejectDisocclusion)
	assertRejected(t, resultAt(t, mask, 5, 5), RejectNormalMismatch)
	if resultAt(t, mask, 0, 6).Rejected() {
		t.Fatal("unchanged object background should remain accepted")
	}
	if stats.NormalMismatch == 0 || stats.Disocclusion == 0 {
		t.Fatalf("object switch reasons were not explicit: %+v", stats)
	}
}

func TestWrongForegroundIsNeverAccepted(t *testing.T) {
	validator, err := NewValidator(DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	current := makeFrame(
		func(x, y int) float32 {
			if x == 1 && y == 1 {
				return 2
			}
			return bgDepth
		},
		func(int, int) float32 { return 1 },
		func(int, int) Vec3 { return facingNormal() },
	)

	mask, stats, err := validator.Evaluate(current, testCamera(), uniformScene(), testCamera())
	if err != nil {
		t.Fatal(err)
	}
	assertConservativeMask(t, mask, stats)
	assertRejected(t, resultAt(t, mask, 1, 1), RejectDisocclusion)
	if stats.Disocclusion == 0 {
		t.Fatalf("wrong foreground was not rejected: %+v", stats)
	}
}
