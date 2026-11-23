package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"headpose"
	"headpose/preview"
)

func main() {
	output := flag.String("out", "artifacts/coordinate-preview.svg", "output SVG path")
	width := flag.Int("width", 1180, "SVG width in pixels")
	height := flag.Int("height", 720, "SVG height in pixels")
	flag.Parse()

	scene, samples, err := buildScene()
	if err != nil {
		fatal(err)
	}
	printContracts()
	if err := printRoundTrips(scene); err != nil {
		fatal(err)
	}
	printSampleConversion(scene)

	if err := os.MkdirAll(filepath.Dir(*output), 0o755); err != nil {
		fatal(fmt.Errorf("create output directory: %w", err))
	}
	file, err := os.Create(*output)
	if err != nil {
		fatal(fmt.Errorf("create preview: %w", err))
	}
	renderErr := preview.RenderSVG(file, preview.Preview{
		Scene:            scene,
		Anchors:          anchors(),
		CameraTrajectory: samples,
	}, preview.Options{
		Title:              "Head-pose frame handoff",
		Width:              *width,
		Height:             *height,
		ScreenWidthPixels:  1280,
		ScreenHeightPixels: 720,
	})
	closeErr := file.Close()
	if renderErr != nil {
		fatal(fmt.Errorf("render preview: %w", renderErr))
	}
	if closeErr != nil {
		fatal(fmt.Errorf("close preview: %w", closeErr))
	}
	fmt.Printf("\npreview: %s\n", *output)
}

func buildScene() (*headpose.Scene, []preview.CameraSample, error) {
	yaw, _ := headpose.QuatFromAxisAngle(headpose.Vec3{Y: 1}, -35*math.Pi/180)
	pitch, _ := headpose.QuatFromAxisAngle(headpose.Vec3{X: 1}, 11*math.Pi/180)
	devicePose, err := headpose.NewRigid(pitch.Mul(yaw), headpose.Vec3{X: -0.35, Y: 1.7, Z: -0.15})
	if err != nil {
		return nil, nil, err
	}

	samples, err := cameraTrajectory(17)
	if err != nil {
		return nil, nil, err
	}
	cameraPose := samples[len(samples)/2].CameraToWorld
	scene, err := headpose.NewScene(devicePose, cameraPose, headpose.ScreenParams{
		PixelsPerMeter: 240,
		PrincipalX:     640,
		PrincipalY:     360,
	})
	return scene, samples, err
}

func cameraTrajectory(count int) ([]preview.CameraSample, error) {
	target := headpose.Vec3{X: 0, Y: 1.62, Z: -1.55}
	samples := make([]preview.CameraSample, 0, count)
	for i := 0; i < count; i++ {
		t := float64(i) / float64(count-1)
		position := headpose.Vec3{
			X: -1.25 + 2.5*t,
			Y: 1.60 + 0.045*math.Sin(2*math.Pi*t),
			Z: 0.48 + 0.08*math.Sin(math.Pi*t),
		}
		transform, err := cameraLookAt(position, target, headpose.Vec3{Y: 1})
		if err != nil {
			return nil, fmt.Errorf("camera sample %d: %w", i, err)
		}
		samples = append(samples, preview.CameraSample{
			Label:         fmt.Sprintf("t=%.2f", t),
			CameraToWorld: transform,
		})
	}
	return samples, nil
}

func cameraLookAt(position, target, up headpose.Vec3) (headpose.Transform, error) {
	forward, err := target.Sub(position).Normalize()
	if err != nil {
		return headpose.Transform{}, fmt.Errorf("look direction: %w", err)
	}
	right, err := forward.Cross(up).Normalize()
	if err != nil {
		return headpose.Transform{}, fmt.Errorf("camera right axis: %w", err)
	}
	backward := forward.Scale(-1)
	cameraUp, err := backward.Cross(right).Normalize()
	if err != nil {
		return headpose.Transform{}, fmt.Errorf("camera up axis: %w", err)
	}
	rotation, err := headpose.QuatFromBasis(right, cameraUp, backward)
	if err != nil {
		return headpose.Transform{}, fmt.Errorf("camera basis: %w", err)
	}
	return headpose.NewRigid(rotation, position)
}

func anchors() []preview.Anchor {
	return []preview.Anchor{
		{Label: "A1", Position: headpose.Vec3{X: -0.95, Y: 1.45, Z: -1.45}, Color: "#ffb454"},
		{Label: "A2", Position: headpose.Vec3{X: 0.00, Y: 1.88, Z: -1.65}, Color: "#ff7b72"},
		{Label: "A3", Position: headpose.Vec3{X: 0.92, Y: 1.52, Z: -1.30}, Color: "#7ee787"},
		{Label: "A4", Position: headpose.Vec3{X: -0.25, Y: 1.15, Z: -1.85}, Color: "#d2a8ff"},
	}
}

func printContracts() {
	fmt.Println("frame contracts")
	for _, spec := range headpose.FrameSpecs() {
		fmt.Printf("  %-7s %-2s  RH  WXYZ  %s | %s | %s\n",
			spec.ID, spec.Unit, spec.XAxis, spec.YAxis, spec.ZAxis)
	}
}

func printRoundTrips(scene *headpose.Scene) error {
	samples := map[headpose.Frame]headpose.Vec3{
		headpose.World:  {X: 0.6, Y: 1.55, Z: -0.8},
		headpose.Device: {X: 0.1, Y: 0.2, Z: -0.25},
		headpose.Camera: {X: 0.2, Y: -0.1, Z: -1.7},
		headpose.Screen: {X: 700, Y: 300, Z: 1200},
	}
	results, err := scene.CheckAllPairs(samples, 1e-8)
	if err != nil {
		return err
	}
	passed := 0
	for _, result := range results {
		if result.Pass {
			passed++
		}
	}
	fmt.Printf("\nround-trip: %d/%d pairs pass · max error %.3g\n", passed, len(results), headpose.MaxRoundTripError(results))
	return nil
}

func printSampleConversion(scene *headpose.Scene) {
	point := headpose.Vec3{X: 0.1, Y: 0.2, Z: -0.25}
	fmt.Println("\ndevice sample (0.1, 0.2, -0.25) m")
	for _, frame := range []headpose.Frame{headpose.World, headpose.Camera, headpose.Screen} {
		converted, err := scene.ConvertPoint(headpose.Device, frame, point)
		if err != nil {
			fatal(err)
		}
		fmt.Printf("  -> %-6s (%8.3f, %8.3f, %8.3f)\n", frame, converted.X, converted.Y, converted.Z)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
