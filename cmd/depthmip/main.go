// Command depthmip renders an analytic reference depth buffer, builds min
// and max reversed-Z mip chains, and verifies every level against the
// analytic base, printing the first erroneous cell and its level if any.
package main

import (
	"fmt"
	"os"

	"depthmip"
)

const (
	width  = 63 // odd on purpose: exercises partial edge blocks
	height = 37
)

func scene() *depthmip.Scene {
	return &depthmip.Scene{
		Spheres: []depthmip.Sphere{
			{C: depthmip.Vec3{-0.4, 0.1, 4}, R: 0.9},
			{C: depthmip.Vec3{0.7, -0.3, 7}, R: 1.4},
		},
		Planes: []depthmip.Plane{{N: depthmip.Vec3{0, 1, 0}, D: 1.2}},
		Near:   0.5,
		Far:    20,
	}
}

func run() error {
	base, err := scene().RenderReference(width, height)
	if err != nil {
		return err
	}
	fmt.Printf("analytic reference: %dx%d reversed-Z (1=near, 0=far)\n", base.W, base.H)

	viewports := []struct {
		name string
		vp   depthmip.Viewport
	}{
		{"full", depthmip.FullViewport(width, height)},
		{"scaled 0.5x centered", depthmip.Viewport{OX: width / 4, OY: height / 4, SX: width / 2, SY: height / 2}},
	}
	for _, name := range []string{"min", "max"} {
		dir, err := depthmip.ParseDirection(name)
		if err != nil {
			return err
		}
		chain, err := base.Chain(dir)
		if err != nil {
			return err
		}
		for _, v := range viewports {
			if err := depthmip.VerifyChain(base, chain, dir, v.vp); err != nil {
				return fmt.Errorf("%s chain, viewport %q: %w", dir, v.name, err)
			}
		}
		fmt.Printf("%s chain: %d levels verified against analytic base (%d viewports)\n",
			dir, len(chain), len(viewports))
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
