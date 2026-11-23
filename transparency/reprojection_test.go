package transparency

import (
	"math"
	"testing"
)

func TestLayerWeightsDependOnDepthAndMotion(t *testing.T) {
	motion := Vec2{X: 0.65, Y: 0.21}
	far := HistoryWeightFor(26, motion)
	near := HistoryWeightFor(1.2, motion)
	fastNear := HistoryWeightFor(1.2, Vec2{X: 1.8, Y: 0.5})

	if math.Abs(far-near) < 0.01 {
		t.Fatalf("different depths shared a weight: far=%f near=%f", far, near)
	}
	if near <= far {
		t.Fatalf("expected near layer weight > far layer weight, got %f <= %f", near, far)
	}
	if fastNear >= near {
		t.Fatalf("faster motion must reduce history weight: fast=%f slow=%f", fastNear, near)
	}
}

func TestCompositeSortsFarToNear(t *testing.T) {
	scene := &Scene{Width: 1, Height: 1}
	bg := scene.BackgroundAt(0, 0)
	farColor := RGB{R: 1}
	nearColor := RGB{B: 1}
	far := &resolvedLayer{
		id: "far", depth: 10, alpha: []float64{0.5},
		premul: []RGB{farColor.Mul(0.5)}, support: []float64{1},
	}
	near := &resolvedLayer{
		id: "near", depth: 1, alpha: []float64{0.5},
		premul: []RGB{nearColor.Mul(0.5)}, support: []float64{1},
	}
	config := Config{
		Width: 1, Height: 1, NearDepthCutoff: 5,
		CoverageCutoff: 0.25, NearAlphaCutoff: 0.05,
	}
	got := Composite(scene, []*resolvedLayer{near, far}, config)
	want := nearColor.Mul(0.5).Add(farColor.Mul(0.25)).Add(bg.Mul(0.25))
	assertRGBClose(t, got.RGB[0], want)
	if got.NearTop[0] != "near" {
		t.Fatalf("near layer should be the top draw-order entry, got %q", got.NearTop[0])
	}
	if math.Abs(got.Alpha[0]-0.75) > 1e-9 {
		t.Fatalf("composite alpha = %f, want 0.75", got.Alpha[0])
	}
}

func TestResolveRejectsCoverageLeftByMovingLayer(t *testing.T) {
	width, height := 4, 1
	current := &LayerImage{
		ID: "glass", Depth: 2, DepthTolerance: 0.5,
		HistoryWeight: 0.7, Motion: Vec2{X: 2},
		Pixels: make([]Surface, width),
	}
	current.Pixels[2] = Surface{Depth: 2, Support: 1, Alpha: 1, Color: RGB{R: 1}}
	prev := &resolvedLayer{
		id: "glass", depth: 2,
		alpha:   []float64{1, 1, 1, 1},
		premul:  []RGB{{R: 1}, {R: 1}, {R: 1}, {R: 1}},
		depthM:  []float64{2, 2, 2, 2},
		support: []float64{1, 1, 1, 1},
	}
	got := ResolveLayers([]*LayerImage{current}, map[string]*resolvedLayer{"glass": prev}, width, height)
	for x := 0; x < 2; x++ {
		if got[0].alpha[x] != 0 || got[0].premul[x] != (RGB{}) {
			t.Fatalf("vacated pixel %d retained old coverage: alpha=%f color=%+v",
				x, got[0].alpha[x], got[0].premul[x])
		}
	}
	if got[0].alpha[2] <= 0 {
		t.Fatal("current covered pixel lost its own contribution")
	}
}

func TestResolveRejectsDepthMismatch(t *testing.T) {
	current := &LayerImage{
		ID: "glass", Depth: 1, DepthTolerance: 0.25,
		HistoryWeight: 0.8, Pixels: []Surface{{Depth: 1, Support: 1}},
	}
	prev := &resolvedLayer{
		id: "glass", depth: 8,
		alpha: []float64{1}, premul: []RGB{{R: 1}},
		depthM: []float64{8}, support: []float64{1},
	}
	got := ResolveLayers([]*LayerImage{current}, map[string]*resolvedLayer{"glass": prev}, 1, 1)
	if got[0].alpha[0] != 0 {
		t.Fatalf("depth-mismatched history leaked into current layer: %f", got[0].alpha[0])
	}
}

func TestExactDepthWithZeroToleranceStaysFinite(t *testing.T) {
	current := &LayerImage{
		ID: "glass", Depth: 1, DepthTolerance: 0, HistoryWeight: 0.8,
		Pixels: []Surface{
			{Depth: 1, Support: 1}, {Depth: 1, Support: 1},
			{Depth: 1, Support: 1}, {Depth: 1, Support: 1},
		},
	}
	prev := &resolvedLayer{
		id: "glass", depth: 1, alpha: []float64{1, 1, 1, 1},
		premul: []RGB{{R: 1}, {R: 1}, {R: 1}, {R: 1}},
		depthM: []float64{1, 1, 1, 1}, support: []float64{1, 1, 1, 1},
	}
	got := ResolveLayers([]*LayerImage{current}, map[string]*resolvedLayer{"glass": prev}, 2, 2)
	if math.IsNaN(got[0].alpha[0]) || math.Abs(got[0].alpha[0]-0.8) > 1e-9 {
		t.Fatalf("unexpected alpha with zero tolerance: %f", got[0].alpha[0])
	}
}

func TestResolveUsesEachLayersOwnHistoryWeight(t *testing.T) {
	fogPixels := []Surface{{Depth: 26, Support: 1}, {Depth: 26, Support: 1}, {Depth: 26, Support: 1}, {Depth: 26, Support: 1}}
	glassPixels := []Surface{{Depth: 4, Support: 1}, {Depth: 4, Support: 1}, {Depth: 4, Support: 1}, {Depth: 4, Support: 1}}
	fog := &LayerImage{
		ID: "fog", Depth: 26, DepthTolerance: 1, HistoryWeight: 0.2,
		Pixels: fogPixels,
	}
	glass := &LayerImage{
		ID: "glass", Depth: 4, DepthTolerance: 1, HistoryWeight: 0.8,
		Pixels: glassPixels,
	}
	prev := map[string]*resolvedLayer{
		"fog": {
			id: "fog", depth: 26, alpha: []float64{1, 1, 1, 1},
			premul: []RGB{{B: 1}, {B: 1}, {B: 1}, {B: 1}},
			depthM: []float64{26, 26, 26, 26}, support: []float64{1, 1, 1, 1},
		},
		"glass": {
			id: "glass", depth: 4, alpha: []float64{1, 1, 1, 1},
			premul: []RGB{{R: 1}, {R: 1}, {R: 1}, {R: 1}},
			depthM: []float64{4, 4, 4, 4}, support: []float64{1, 1, 1, 1},
		},
	}
	got := ResolveLayers([]*LayerImage{fog, glass}, prev, 2, 2)
	if math.Abs(got[0].alpha[0]-0.2) > 1e-9 {
		t.Fatalf("fog used the wrong history weight: alpha=%f", got[0].alpha[0])
	}
	if math.Abs(got[1].alpha[0]-0.8) > 1e-9 {
		t.Fatalf("glass used the wrong history weight: alpha=%f", got[1].alpha[0])
	}
	assertRGBClose(t, got[0].premul[0], RGB{B: 0.2})
	assertRGBClose(t, got[1].premul[0], RGB{R: 0.8})
}

func TestPerLayerComparisonBeatsBlockAndNoHistory(t *testing.T) {
	config := Config{Width: 64, Height: 40, Frames: 16, ReferenceSamples: 4}
	evaluation := Evaluate(NewDemoScene(config.Width, config.Height), config)
	per := evaluation.Summary[StrategyPerLayer]
	block := evaluation.Summary[StrategyWholeBlock]
	none := evaluation.Summary[StrategyNoHistory]

	if per.Metrics.ColorRMSE >= block.Metrics.ColorRMSE {
		t.Fatalf("per-layer color error %.6f not better than block %.6f",
			per.Metrics.ColorRMSE, block.Metrics.ColorRMSE)
	}
	if per.Metrics.ColorRMSE >= none.Metrics.ColorRMSE {
		t.Fatalf("per-layer color error %.6f not better than no-history %.6f",
			per.Metrics.ColorRMSE, none.Metrics.ColorRMSE)
	}
	if per.Metrics.CoverageIoU <= block.Metrics.CoverageIoU {
		t.Fatalf("per-layer coverage IoU %.6f not better than block %.6f",
			per.Metrics.CoverageIoU, block.Metrics.CoverageIoU)
	}
	if per.Metrics.StaleCoverage != 0 {
		t.Fatalf("per-layer path retained stale coverage: %f", per.Metrics.StaleCoverage)
	}
	if block.Metrics.StaleCoverage <= per.Metrics.StaleCoverage {
		t.Fatalf("block path should expose stale coverage, block=%f per=%f",
			block.Metrics.StaleCoverage, per.Metrics.StaleCoverage)
	}
	if per.Metrics.OrderAgreement <= block.Metrics.OrderAgreement {
		t.Fatalf("per-layer order agreement %.6f not better than block %.6f",
			per.Metrics.OrderAgreement, block.Metrics.OrderAgreement)
	}
}

func assertRGBClose(t *testing.T, got, want RGB) {
	t.Helper()
	if math.Abs(got.R-want.R) > 1e-9 ||
		math.Abs(got.G-want.G) > 1e-9 ||
		math.Abs(got.B-want.B) > 1e-9 {
		t.Fatalf("RGB mismatch: got %+v want %+v", got, want)
	}
}
