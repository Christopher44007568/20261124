package offaxis

import (
	"math"
	"testing"
)

func TestProjectionMatchesFrustumCorners(t *testing.T) {
	b := Bounds{Left: -1.7, Right: 0.9, Bottom: -1.1, Top: 0.6, Near: 2, Far: 12}
	m, err := b.ProjectionMatrix()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	corners, err := b.FrustumCorners()
	if err != nil {
		t.Fatal(err)
	}
	wantX := map[float64]float64{b.Left: -1, b.Right: 1}
	wantY := map[float64]float64{b.Bottom: -1, b.Top: 1}
	for _, c := range corners {
		ndc := m.TransformPoint(c).NDC()
		nearPlane := math.Abs(c.Z+b.Near) < 1e-9
		if got := wantX[nearBoundX(b, c)]; !approx(ndc.X, got, 1e-9) {
			t.Fatalf("corner %+v ndc.X=%v want %v", c, ndc.X, got)
		}
		if got := wantY[nearBoundY(b, c)]; !approx(ndc.Y, got, 1e-9) {
			t.Fatalf("corner %+v ndc.Y=%v want %v", c, ndc.Y, got)
		}
		wantZ := 1.0
		if nearPlane {
			wantZ = -1
		}
		if !approx(ndc.Z, wantZ, 1e-9) {
			t.Fatalf("corner %+v ndc.Z=%v want %v", c, ndc.Z, wantZ)
		}
	}
}

func nearBoundX(b Bounds, c Vec3) float64 {
	if c.X < 0 {
		return b.Left
	}
	return b.Right
}

func nearBoundY(b Bounds, c Vec3) float64 {
	if c.Y < 0 {
		return b.Bottom
	}
	return b.Top
}

func TestRejectionCases(t *testing.T) {
	cases := []struct {
		name string
		b    Bounds
		want error
	}{
		{"zero near", Bounds{-1, 1, -1, 1, 0, 10}, ErrInvalidDepth},
		{"negative far", Bounds{-1, 1, -1, 1, 2, -10}, ErrInvalidDepth},
		{"reversed depth", Bounds{-1, 1, -1, 1, 8, 4}, ErrReversedDepth},
		{"equal depth", Bounds{-1, 1, -1, 1, 4, 4}, ErrReversedDepth},
		{"degenerate x", Bounds{1, 1, -1, 1, 2, 10}, ErrDegenerate},
		{"degenerate y", Bounds{-1, 1, 2, 2, 2, 10}, ErrDegenerate},
		{"nan bound", Bounds{math.NaN(), 1, -1, 1, 2, 10}, ErrInvalidDepth},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tc.b.ProjectionMatrix(); err != tc.want {
				t.Fatalf("got %v want %v", err, tc.want)
			}
		})
	}
}

func TestOffAxisTerms(t *testing.T) {
	b := Bounds{Left: 0.2, Right: 1.4, Bottom: -0.5, Top: 0.7, Near: 1, Far: 10}
	m, err := b.ProjectionMatrix()
	if err != nil {
		t.Fatal(err)
	}
	wantTx := (b.Right + b.Left) / (b.Right - b.Left)
	wantTy := (b.Top + b.Bottom) / (b.Top - b.Bottom)
	if !approx(m[0][2], wantTx, 1e-12) || !approx(m[1][2], wantTy, 1e-12) {
		t.Fatalf("off-axis terms wrong: %+v", m)
	}
	// The viewport-shift-only alternative (zero third column) must fail the
	// center-ray check, demonstrating why shifting cannot fix off-axis FOV.
	broken := m
	broken[0][2] = 0
	if err := broken.checkCenterRay(b); err != ErrCenterRay {
		t.Fatalf("shift-only matrix should be rejected, got %v", err)
	}
}
