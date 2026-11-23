package headpose

import (
	"errors"
	"math"
	"testing"
)

func TestFrameGraphPathAndRoundTrip(t *testing.T) {
	graph := NewFrameGraph()
	deviceToWorld := mustRigid(t, mustQuat(t, Vec3{Y: 1}, 90), Vec3{X: 1, Y: 2})
	if err := graph.AddTransform(Device, World, deviceToWorld); err != nil {
		t.Fatalf("AddTransform: %v", err)
	}

	path, err := graph.Path(Device, World)
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if len(path) != 2 || path[0] != Device || path[1] != World {
		t.Fatalf("path = %v", path)
	}

	point := Vec3{X: 0.5, Y: -1, Z: 2}
	result, err := graph.RoundTripPoint(Device, World, point, 1e-12)
	if err != nil {
		t.Fatalf("RoundTripPoint: %v", err)
	}
	if !result.Pass {
		t.Fatalf("round trip failed: %+v", result)
	}
	if result.MaxError > 1e-12 {
		t.Fatalf("round-trip error = %g", result.MaxError)
	}
	if MaxRoundTripError([]RoundTripResult{result}) != result.MaxError {
		t.Fatal("MaxRoundTripError returned the wrong value")
	}
}

func TestFrameGraphMissingPath(t *testing.T) {
	graph := NewFrameGraph()
	_, err := graph.ConvertPoint(World, Screen, Vec3{})
	if !errors.Is(err, ErrNoPath) {
		t.Fatalf("got error %v, want ErrNoPath", err)
	}
	path, err := graph.Path(World, World)
	if err != nil || len(path) != 1 {
		t.Fatalf("identity path = %v, %v", path, err)
	}
}

func TestFrameGraphRejectsInvalidEdges(t *testing.T) {
	graph := NewFrameGraph()
	if err := graph.AddTransform("bogus", World, IdentityTransform()); err == nil {
		t.Fatal("expected unknown frame to fail")
	}
	if err := graph.AddTransform(World, World, IdentityTransform()); err == nil {
		t.Fatal("expected same-frame edge to fail")
	}
	if err := graph.AddTransform(World, Device, Transform{}); err == nil {
		t.Fatal("expected zero transform to fail")
	}
}

func TestCheckAllPairsRejectsMissingSample(t *testing.T) {
	scene := testScene(t)
	if _, err := scene.CheckAllPairs(map[Frame]Vec3{World: {}}, 1e-9); err == nil {
		t.Fatal("expected missing frame samples to fail")
	}
}

func TestCheckAllPairsReportsFiniteErrors(t *testing.T) {
	scene := testScene(t)
	samples := map[Frame]Vec3{
		World:  {X: 1.2, Y: 1.7, Z: 2.4},
		Device: {X: 0.2, Y: 0.1, Z: -0.4},
		Camera: {X: 0.3, Y: -0.2, Z: -2.0},
		Screen: {X: 700, Y: 300, Z: 1200},
	}
	results, err := scene.CheckAllPairs(samples, 1e-8)
	if err != nil {
		t.Fatalf("CheckAllPairs: %v", err)
	}
	if len(results) != 16 {
		t.Fatalf("got %d results, want 16", len(results))
	}
	for _, result := range results {
		if !result.Pass {
			t.Errorf("%s -> %s failed with error %g", result.From, result.To, result.MaxError)
		}
	}
	if math.IsNaN(MaxRoundTripError(results)) {
		t.Fatal("max round-trip error must not be NaN")
	}
}
