package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"

	"binocular"
)

func main() {
	output := flag.String("out", "artifacts/demo", "output directory")
	flag.Parse()

	cube := cubeGeometry()
	probe := &binocular.Geometry{
		Name: "near-clip-probe",
		Vertices: []binocular.Vertex{
			{Position: binocular.Vec3{X: -0.16, Y: -0.16}, Color: binocular.Vec3{X: 1, Y: 0.3, Z: 0.2}},
			{Position: binocular.Vec3{X: 0.16, Y: -0.16}, Color: binocular.Vec3{X: 0.2, Y: 1, Z: 0.4}},
			{Position: binocular.Vec3{X: 0, Y: 0.16, Z: 0.8}, Color: binocular.Vec3{X: 0.3, Y: 0.4, Z: 1}},
		},
		Indices: []uint32{0, 1, 2},
	}
	clear := binocular.RGB{R: 7, G: 9, B: 15}
	leftCamera := binocular.NewPerspectiveCamera(
		binocular.Vec3{X: -0.18, Z: 3.4}, binocular.Vec3{}, binocular.Vec3{Y: 1},
		math.Pi/180*55, 1, 0.5, 50, 96, 96, clear,
	)
	rightCamera := binocular.NewPerspectiveCamera(
		binocular.Vec3{X: 0.18, Z: 3.4}, binocular.Vec3{}, binocular.Vec3{Y: 1},
		math.Pi/180*55, 1, 0.5, 50, 96, 96, clear,
	)

	left := binocular.EyePass{
		Eye:    binocular.EyeLeft,
		Camera: leftCamera,
		Items: []binocular.DrawItem{
			{
				Geometry: cube,
				Model: binocular.Translate(0, 0, 0).
					Mul(binocular.RotateY(0.38)).
					Mul(binocular.Scale(0.85, 0.85, 0.85)),
				Material: binocular.Material{BaseColor: binocular.Vec3{X: 1.0, Y: 0.62, Z: 0.42}},
			},
			{
				Geometry: probe,
				Model:    binocular.Translate(0.18, -0.38, 2.35).Mul(binocular.RotateY(0.2)),
				Material: binocular.Material{BaseColor: binocular.Vec3{X: 1.0, Y: 0.85, Z: 0.7}},
			},
		},
	}
	right := binocular.EyePass{
		Eye:    binocular.EyeRight,
		Camera: rightCamera,
		Items: []binocular.DrawItem{
			{
				Geometry: cube,
				Model: binocular.Translate(0, 0, 0).
					Mul(binocular.RotateY(-0.34)).
					Mul(binocular.Scale(0.85, 0.85, 0.85)),
				Material: binocular.Material{BaseColor: binocular.Vec3{X: 0.42, Y: 0.62, Z: 1.0}},
			},
			{
				Geometry: probe,
				Model:    binocular.Translate(0.5, -0.55, 0.2).Mul(binocular.RotateY(-0.2)),
				Material: binocular.Material{BaseColor: binocular.Vec3{X: 0.7, Y: 0.85, Z: 1.0}},
			},
		},
	}

	rasterizer := binocular.NewRasterizer()
	shared, err := rasterizer.RenderShared(left, right)
	if err != nil {
		log.Fatal(err)
	}
	reference, err := rasterizer.RenderReconstructed(left, right)
	if err != nil {
		log.Fatal(err)
	}
	comparison := binocular.CompareResults(shared, reference)

	if err := os.MkdirAll(*output, 0o755); err != nil {
		log.Fatal(err)
	}
	writePPM(filepath.Join(*output, "shared_left.ppm"), shared.Left)
	writePPM(filepath.Join(*output, "shared_right.ppm"), shared.Right)
	writePPM(filepath.Join(*output, "reference_left.ppm"), reference.Left)
	writePPM(filepath.Join(*output, "reference_right.ppm"), reference.Right)
	writeJSON(filepath.Join(*output, "report.json"), struct {
		SharedStats    binocular.StereoStats `json:"shared_stats"`
		ReferenceStats binocular.StereoStats `json:"reference_stats"`
		Comparison     binocular.Comparison  `json:"comparison"`
		Images         map[string]string     `json:"images"`
	}{
		SharedStats:    shared.Stats,
		ReferenceStats: reference.Stats,
		Comparison:     comparison,
		Images: map[string]string{
			"shared_left":     "shared_left.ppm",
			"shared_right":    "shared_right.ppm",
			"reference_left":  "reference_left.ppm",
			"reference_right": "reference_right.ppm",
		},
	})

	fmt.Printf("shared batches: left=%d right=%d total=%d\n",
		shared.Stats.Left.Batches, shared.Stats.Right.Batches, shared.Stats.Batches)
	fmt.Printf("shared geometry accesses: %d (left=%d right=%d)\n",
		shared.Stats.SharedGeometryAccesses,
		shared.Stats.Left.GeometryAccesses, shared.Stats.Right.GeometryAccesses)
	fmt.Printf("clipped triangles: left=%d right=%d\n",
		shared.Stats.Left.ClippedTriangles, shared.Stats.Right.ClippedTriangles)
	fmt.Printf("geometry upload: shared=%d B / %d upload(s), reference=%d B / %d upload(s), saved=%d B\n",
		shared.Stats.GeometryUploadBytes, shared.Stats.GeometryUploads,
		reference.Stats.GeometryUploadBytes, reference.Stats.GeometryUploads,
		comparison.GeometryUploadBytesSaved)
	fmt.Printf("transform upload: shared=%d B, reference=%d B\n",
		shared.Stats.TransformUploadBytes, reference.Stats.TransformUploadBytes)
	fmt.Printf("left/right transform digest: %016x / %016x\n",
		shared.Stats.Left.TransformDigest, shared.Stats.Right.TransformDigest)
	fmt.Printf("matches reconstructed reference: %t\n", comparison.ExactMatch)
	if !comparison.ExactMatch {
		log.Fatalf("comparison failed: %+v", comparison)
	}
}

func cubeGeometry() *binocular.Geometry {
	positions := [...]binocular.Vec3{
		{X: -1, Y: -1, Z: -1}, {X: 1, Y: -1, Z: -1},
		{X: 1, Y: 1, Z: -1}, {X: -1, Y: 1, Z: -1},
		{X: -1, Y: -1, Z: 1}, {X: 1, Y: -1, Z: 1},
		{X: 1, Y: 1, Z: 1}, {X: -1, Y: 1, Z: 1},
	}
	faces := [...][4]int{
		{0, 1, 2, 3}, {5, 4, 7, 6}, {4, 0, 3, 7},
		{1, 5, 6, 2}, {3, 2, 6, 7}, {4, 5, 1, 0},
	}
	geometry := &binocular.Geometry{Name: "shared-cube"}
	for _, face := range faces {
		base := uint32(len(geometry.Vertices))
		for corner, positionIndex := range face {
			shade := 0.45 + float64(corner)*0.12
			geometry.Vertices = append(geometry.Vertices, binocular.Vertex{
				Position: positions[positionIndex],
				Color:    binocular.Vec3{X: shade, Y: shade, Z: shade},
			})
		}
		geometry.Indices = append(geometry.Indices,
			base, base+1, base+2,
			base, base+2, base+3,
		)
	}
	return geometry
}

func writePPM(path string, image binocular.Image) {
	file, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	if err := image.WritePPM(file); err != nil {
		log.Fatal(err)
	}
}

func writeJSON(path string, value any) {
	file, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		log.Fatal(err)
	}
}
