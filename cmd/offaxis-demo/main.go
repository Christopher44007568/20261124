package main

import (
	"errors"
	"fmt"
	"os"

	"offaxis/pkg/offaxis"
)

func main() {
	fmt.Println("off-axis per-eye projection + software rasterizer cross-check")
	fmt.Println(planeLegend)

	failed := false
	for _, s := range scenarios() {
		if !runScenario(s) {
			failed = true
		}
	}

	fmt.Println("\n=== 5. rejection rules ===")
	if !expectRejected("reversed near/far",
		offaxis.Bounds{Left: -1, Right: 1, Bottom: -1, Top: 1, Near: 8, Far: 4},
		offaxis.ErrReversedDepth) {
		failed = true
	}
	if !expectRejected("non-positive near",
		offaxis.Bounds{Left: -1, Right: 1, Bottom: -1, Top: 1, Near: 0, Far: 4},
		offaxis.ErrInvalidDepth) {
		failed = true
	}

	le := offaxis.Eye{Name: "L", Bounds: offaxis.Bounds{Left: -1, Right: 1, Bottom: -1, Top: 1, Near: 1, Far: 10}}
	re := offaxis.Eye{Name: "R", Bounds: offaxis.Bounds{Left: -1, Right: 1, Bottom: -1, Top: 1, Near: 1, Far: 10}}
	yawed := offaxis.Identity()
	yawed[0][0] = 0.999
	yawed[0][2] = 0.044
	yawed[2][0] = -0.044
	yawed[2][2] = 0.999
	re.Pose.Rotation = yawed
	if _, _, err := offaxis.BuildStereo(le, re); !errors.Is(err, offaxis.ErrCenterRay) {
		fmt.Printf("toe-in stereo: expected ErrCenterRay, got %v\n", err)
		failed = true
	} else {
		fmt.Println("toe-in stereo (converging center rays): REJECTED with ErrCenterRay")
	}

	// Tampered matrix: zeroing the off-axis term must fail the ray check.
	b := offaxis.Bounds{Left: 0.2, Right: 1.4, Bottom: -0.5, Top: 0.7, Near: 1, Far: 10}
	good, err := b.ProjectionMatrix()
	if err != nil {
		fmt.Printf("reference matrix failed: %v\n", err)
		failed = true
	} else {
		broken := good
		broken[0][2] = 0
		if err := broken.VerifyCenterRay(b); !errors.Is(err, offaxis.ErrCenterRay) {
			fmt.Printf("tampered matrix: expected ErrCenterRay, got %v\n", err)
			failed = true
		} else {
			fmt.Println("viewport-shift-only matrix (off-axis term removed): REJECTED with ErrCenterRay")
		}
	}

	if failed {
		fmt.Println("\nRESULT: FAIL")
		os.Exit(1)
	}
	fmt.Println("\nRESULT: all cross-checks passed")
}

func expectRejected(label string, b offaxis.Bounds, want error) bool {
	_, err := b.ProjectionMatrix()
	if errors.Is(err, want) {
		fmt.Printf("%s: REJECTED with %v\n", label, want)
		return true
	}
	fmt.Printf("%s: expected %v, got %v\n", label, want, err)
	return false
}
