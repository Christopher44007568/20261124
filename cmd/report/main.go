// Command report prints reprojection and disparity errors for each
// simulated calibration fault, showing which marker family exposes
// it and whether a whole-image translation can hide it.
package main

import (
	"fmt"

	"centercal/calib"
)

func main() {
	rig := calib.DefaultRig()
	m := rig.DefaultMarkers()
	all := m.All()
	scenarios := []struct {
		name string
		pert calib.Perturbation
	}{
		{"clean", calib.Perturbation{}},
		{"pure translation (7.5,-3.25)px", calib.Perturbation{
			Translation: calib.Vec2{X: 7.5, Y: -3.25}}},
		{"rotation 0.23deg about panel center", calib.Perturbation{
			Rotation: 0.004, Pivot: rig.Centers.Panel}},
		{"rotation 0.23deg about render center", calib.Perturbation{
			Rotation: 0.004, Pivot: rig.Centers.Render}},
		{"viewpoint shift +4mm both eyes", calib.Perturbation{
			ViewpointShift: [2]calib.Vec3{{X: 0.004}, {X: 0.004}}}},
		{"left-eye bias (6,-2.5)px", calib.Perturbation{
			EyeBias: [2]calib.Vec2{{X: 6, Y: -2.5}}}},
		{"right-eye bias (6,-2.5)px", calib.Perturbation{
			EyeBias: [2]calib.Vec2{{}, {X: 6, Y: -2.5}}}},
		{"opposite eye biases +-(4,1.5)px", calib.Perturbation{
			EyeBias: [2]calib.Vec2{{X: 4, Y: 1.5}, {X: -4, Y: -1.5}}}},
	}
	fmt.Printf("%-38s %-18s %10s %10s %10s\n",
		"scenario", "diagnosis", "leftover", "disparity", "depth-sens")
	for _, s := range scenarios {
		d := rig.Diagnose(all, s.pert, 0.5)
		depth := calib.DepthSensitivity(rig.ObserveAll(m.Depth, calib.Left, s.pert))
		fmt.Printf("%-38s %-18s %9.3fpx %9.3fpx %9.3fpx\n",
			s.name, d.Kind, d.SharedLeftover, d.Disparity, depth)
	}
	fmt.Println("\ncenter confusion (RMS px, boundary markers, left eye):")
	for _, actual := range calib.CenterKinds() {
		for _, assumed := range calib.CenterKinds() {
			if assumed == actual {
				continue
			}
			err := rig.ConfusionError(m.Boundary, calib.Left, assumed, actual)
			fmt.Printf("  %s used as %s: %.3f px -> %s\n",
				assumed, actual, err, verdict(err))
		}
	}
}

func verdict(err float64) string {
	if err > 0.5 {
		return "FLAGGED (correct)"
	}
	return "MISSED (bug)"
}
