package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type testMetrics struct {
	Cases []struct {
		ID       string `json:"id"`
		Rotation struct {
			MAE           float64 `json:"mae"`
			Coverage      float64 `json:"coverage"`
			OutOfBounds   int     `json:"out_of_bounds"`
			SourceNoDepth int     `json:"source_no_depth"`
			Regions       []struct {
				Name     string  `json:"name"`
				MAE      float64 `json:"mae"`
				Coverage float64 `json:"coverage"`
			} `json:"regions"`
		} `json:"pure_rotation"`
		Depth struct {
			MAE           float64 `json:"mae"`
			Coverage      float64 `json:"coverage"`
			OutOfBounds   int     `json:"out_of_bounds"`
			SourceNoDepth int     `json:"source_no_depth"`
			Regions       []struct {
				Name     string  `json:"name"`
				MAE      float64 `json:"mae"`
				Coverage float64 `json:"coverage"`
			} `json:"regions"`
		} `json:"depth_reprojection"`
	} `json:"cases"`
}

func TestBuildEndToEndInvariants(t *testing.T) {
	dir := t.TempDir()
	if err := build(dir, 192, 108); err != nil {
		t.Fatal(err)
	}
	buf, err := os.ReadFile(filepath.Join(dir, "metrics.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m testMetrics
	if err := json.Unmarshal(buf, &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Cases) != 2 {
		t.Fatalf("cases = %d", len(m.Cases))
	}
	turn, dolly := m.Cases[0], m.Cases[1]
	if turn.Rotation.MAE > 5 {
		t.Fatalf("turn rotation should nearly match GT, MAE=%g", turn.Rotation.MAE)
	}
	if turn.Rotation.OutOfBounds == 0 {
		t.Fatal("fast turn should reject out-of-frame rays")
	}
	if turn.Depth.OutOfBounds != 0 || dolly.Depth.OutOfBounds != 0 {
		t.Fatalf("depth path sampled out of bounds: turn=%d dolly=%d", turn.Depth.OutOfBounds, dolly.Depth.OutOfBounds)
	}
	if turn.Depth.SourceNoDepth == 0 {
		t.Fatal("expected the source background depth dropout")
	}
	if turn.Rotation.SourceNoDepth != 0 {
		t.Fatal("rotation path reported depth use")
	}
	if dolly.Rotation.Coverage < 0.999 {
		t.Fatalf("pure rotation on dolly should have full coverage, got %g", dolly.Rotation.Coverage)
	}
	rotPole := regionInTest(dolly.Rotation.Regions, "near pole")
	depPole := regionInTest(dolly.Depth.Regions, "near pole")
	if rotPole.MAE < depPole.MAE*4 {
		t.Fatalf("dolly near pole should expose missing parallax: rotation %g vs depth %g", rotPole.MAE, depPole.MAE)
	}
	if dolly.Depth.Coverage < 0.70 {
		t.Fatalf("depth dolly coverage too low: %g", dolly.Depth.Coverage)
	}
}

func regionInTest(regions []struct {
	Name     string  `json:"name"`
	MAE      float64 `json:"mae"`
	Coverage float64 `json:"coverage"`
}, name string) struct {
	Name     string  `json:"name"`
	MAE      float64 `json:"mae"`
	Coverage float64 `json:"coverage"`
} {
	for _, r := range regions {
		if r.Name == name {
			return r
		}
	}
	return struct {
		Name     string  `json:"name"`
		MAE      float64 `json:"mae"`
		Coverage float64 `json:"coverage"`
	}{}
}
