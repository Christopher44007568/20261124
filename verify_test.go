package depthmip

import (
	"errors"
	"strings"
	"testing"
)

func testScene() *Scene {
	return &Scene{
		Spheres: []Sphere{
			{C: Vec3{-0.4, 0.1, 4}, R: 0.9},
			{C: Vec3{0.7, -0.3, 7}, R: 1.4},
		},
		Planes: []Plane{{N: Vec3{0, 1, 0}, D: 1.2}},
		Near:   0.5,
		Far:    20,
	}
}

func TestAnalyticReferenceBothDirections(t *testing.T) {
	base, err := testScene().RenderReference(63, 37)
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []Direction{Min, Max} {
		chain, err := base.Chain(dir)
		if err != nil {
			t.Fatalf("%s: %v", dir, err)
		}
		if err := VerifyChain(base, chain, dir, FullViewport(63, 37)); err != nil {
			t.Errorf("%s: %v", dir, err)
		}
	}
}

func TestVerifyReportsFirstBadCellAndLevel(t *testing.T) {
	base, err := testScene().RenderReference(63, 37)
	if err != nil {
		t.Fatal(err)
	}
	chain, err := base.Chain(Min)
	if err != nil {
		t.Fatal(err)
	}
	// Corrupt a level-2 cell; level 1 stays intact so the report must
	// identify level 2 as the source.
	chain[2].Data[3*chain[2].W+5] += 0.25
	err = VerifyChain(base, chain, Min, FullViewport(63, 37))
	var ce *CellError
	if !errors.As(err, &ce) {
		t.Fatalf("want *CellError, got %v", err)
	}
	if ce.Level != 2 || ce.X != 5 || ce.Y != 3 {
		t.Errorf("got level %d cell (%d,%d), want level 2 cell (5,3)", ce.Level, ce.X, ce.Y)
	}
}

func TestViewportRestrictsVerification(t *testing.T) {
	base, err := testScene().RenderReference(64, 64)
	if err != nil {
		t.Fatal(err)
	}
	chain, err := base.Chain(Max)
	if err != nil {
		t.Fatal(err)
	}
	// Corrupt a level-1 cell in the bottom-right quarter, then verify only
	// the top-left quarter viewport: the corruption must stay invisible.
	chain[1].Data[20*chain[1].W+20] = 0
	vp := Viewport{OX: 0, OY: 0, SX: 16, SY: 16}
	if err := VerifyChain(base, chain, Max, vp); err != nil {
		t.Errorf("corruption outside viewport leaked in: %v", err)
	}
	// A viewport covering the corrupted cell must surface it.
	vp = Viewport{OX: 32, OY: 32, SX: 32, SY: 32}
	var ce *CellError
	if err := VerifyChain(base, chain, Max, vp); !errors.As(err, &ce) {
		t.Fatalf("want *CellError for visible corruption, got %v", err)
	} else if ce.Level != 1 || ce.X != 20 || ce.Y != 20 {
		t.Errorf("got level %d cell (%d,%d), want level 1 cell (20,20)", ce.Level, ce.X, ce.Y)
	}
}

func TestInvalidViewportRejected(t *testing.T) {
	base, err := testScene().RenderReference(8, 8)
	if err != nil {
		t.Fatal(err)
	}
	chain, err := base.Chain(Min)
	if err != nil {
		t.Fatal(err)
	}
	err = VerifyChain(base, chain, Min, Viewport{SX: -1, SY: 8})
	if err == nil || !strings.Contains(err.Error(), "viewport") {
		t.Errorf("negative viewport scale: want viewport error, got %v", err)
	}
}
