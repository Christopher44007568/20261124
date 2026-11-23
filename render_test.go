package binocular

import (
	"math"
	"testing"
)

func testQuad() *Geometry {
	return &Geometry{
		Name: "quad",
		Vertices: []Vertex{
			{Position: Vec3{-1, -1, 0}, Color: Vec3{0.35, 0.35, 0.35}},
			{Position: Vec3{1, -1, 0}, Color: Vec3{0.70, 0.70, 0.70}},
			{Position: Vec3{1, 1, 0}, Color: Vec3{0.95, 0.95, 0.95}},
			{Position: Vec3{-1, 1, 0}, Color: Vec3{0.55, 0.55, 0.55}},
		},
		Indices: []uint32{0, 1, 2, 0, 2, 3},
	}
}

func testCamera(x float64) Camera {
	return NewPerspectiveCamera(
		Vec3{X: x, Y: 0, Z: 3},
		Vec3{},
		Vec3{Y: 1},
		math.Pi/3,
		1,
		0.1,
		100,
		64,
		64,
		RGB{R: 8, G: 10, B: 16},
	)
}

func testPass(eye Eye, camera Camera, geometry *Geometry, material Material, model Mat4) EyePass {
	return EyePass{
		Eye:    eye,
		Camera: camera,
		Items: []DrawItem{{
			Geometry: geometry,
			Model:    model,
			Material: material,
		}},
	}
}

func testStereoPasses(geometry *Geometry) (EyePass, EyePass) {
	left := testPass(EyeLeft, testCamera(-0.15), geometry,
		Material{BaseColor: Vec3{1.0, 0.45, 0.25}}, RotateY(0.18))
	right := testPass(EyeRight, testCamera(0.15), geometry,
		Material{BaseColor: Vec3{0.25, 0.55, 1.0}}, RotateY(-0.22))
	return left, right
}

func TestSharedMatchesReconstructedReference(t *testing.T) {
	geometry := testQuad()
	left, right := testStereoPasses(geometry)
	rasterizer := NewRasterizer()

	shared, err := rasterizer.RenderShared(left, right)
	if err != nil {
		t.Fatalf("shared render: %v", err)
	}
	reference, err := rasterizer.RenderReconstructed(left, right)
	if err != nil {
		t.Fatalf("reference render: %v", err)
	}
	comparison := CompareResults(shared, reference)

	if !comparison.ExactMatch {
		t.Fatalf("shared and reconstructed renders differ: %+v", comparison)
	}
	if !shared.Left.Equal(reference.Left) || !shared.Right.Equal(reference.Right) {
		t.Fatal("pixel buffers differ despite ExactMatch")
	}
	if shared.Stats.Left.Batches != 1 || shared.Stats.Right.Batches != 1 {
		t.Fatalf("batches = left:%d right:%d, want 1/1",
			shared.Stats.Left.Batches, shared.Stats.Right.Batches)
	}
	if shared.Stats.SharedGeometryAccesses != 2 {
		t.Fatalf("shared geometry accesses = %d, want 2",
			shared.Stats.SharedGeometryAccesses)
	}
	if reference.Stats.SharedGeometryAccesses != 0 {
		t.Fatalf("reference shared geometry accesses = %d, want 0",
			reference.Stats.SharedGeometryAccesses)
	}
	if shared.Stats.GeometryUploads != 1 || reference.Stats.GeometryUploads != 2 {
		t.Fatalf("geometry uploads = shared:%d reference:%d, want 1/2",
			shared.Stats.GeometryUploads, reference.Stats.GeometryUploads)
	}
	if reference.Stats.GeometryUploadBytes != 2*geometry.UploadBytes() {
		t.Fatalf("reference geometry upload bytes = %d, want %d",
			reference.Stats.GeometryUploadBytes, 2*geometry.UploadBytes())
	}
	if reference.Stats.ReconstructedGeometryCopies != 2 {
		t.Fatalf("reconstructed copies = %d, want 2",
			reference.Stats.ReconstructedGeometryCopies)
	}
	if shared.Stats.TransformUploads != 6 ||
		shared.Stats.TransformUploadBytes != 6*16*8 {
		t.Fatalf("transform uploads = %d/%d, want 6/%d",
			shared.Stats.TransformUploads, shared.Stats.TransformUploadBytes, 6*16*8)
	}
	if comparison.GeometryUploadBytesSaved != int64(geometry.UploadBytes()) {
		t.Fatalf("geometry bytes saved = %d, want %d",
			comparison.GeometryUploadBytesSaved, geometry.UploadBytes())
	}
}

func TestPerEyeStateIsIsolated(t *testing.T) {
	geometry := testQuad()
	baselineLeft, baselineRight := testStereoPasses(geometry)
	baseline, err := RenderShared(baselineLeft, baselineRight)
	if err != nil {
		t.Fatalf("baseline render: %v", err)
	}

	materialOnly := baselineRight
	materialOnly.Items = append([]DrawItem(nil), baselineRight.Items...)
	materialOnly.Items[0].Material = Material{BaseColor: Vec3{0.1, 1.0, 0.1}}
	changedMaterial, err := RenderShared(baselineLeft, materialOnly)
	if err != nil {
		t.Fatalf("material render: %v", err)
	}
	if !baseline.Left.Equal(changedMaterial.Left) {
		t.Fatal("right-eye material change altered left-eye pixels")
	}
	if baseline.Right.Equal(changedMaterial.Right) {
		t.Fatal("right-eye material change did not alter right-eye pixels")
	}

	rotationOnly := baselineRight
	rotationOnly.Items = append([]DrawItem(nil), baselineRight.Items...)
	rotationOnly.Items[0].Model = RotateY(0.75)
	changedRotation, err := RenderShared(baselineLeft, rotationOnly)
	if err != nil {
		t.Fatalf("rotation render: %v", err)
	}
	if !baseline.Left.Equal(changedRotation.Left) {
		t.Fatal("right-eye instance rotation altered left-eye pixels")
	}
	if baseline.Right.Equal(changedRotation.Right) {
		t.Fatal("right-eye instance rotation did not alter right-eye pixels")
	}

	cameraOnly := baselineRight
	cameraOnly.Camera = testCamera(0.65)
	changedCamera, err := RenderShared(baselineLeft, cameraOnly)
	if err != nil {
		t.Fatalf("camera render: %v", err)
	}
	if !baseline.Left.Equal(changedCamera.Left) {
		t.Fatal("right-eye camera change altered left-eye pixels")
	}
	if baseline.Left.Equal(baseline.Right) {
		t.Fatal("left and right eye outputs are unexpectedly identical")
	}
}

func TestClippingIsMonocularAndMatchesReference(t *testing.T) {
	geometry := &Geometry{
		Name: "near-clip",
		Vertices: []Vertex{
			{Position: Vec3{-1, -1, 0}, Color: Vec3{1, 0, 0}},
			{Position: Vec3{1, -1, 0}, Color: Vec3{0, 1, 0}},
			{Position: Vec3{0, 0.5, 2.5}, Color: Vec3{0, 0, 1}},
		},
		Indices: []uint32{0, 1, 2},
	}
	leftCamera := NewPerspectiveCamera(
		Vec3{Z: 2}, Vec3{}, Vec3{Y: 1}, math.Pi/3, 1, 0.1, 100,
		64, 64, RGB{},
	)
	rightCamera := NewPerspectiveCamera(
		Vec3{Z: 4}, Vec3{}, Vec3{Y: 1}, math.Pi/3, 1, 0.1, 100,
		64, 64, RGB{},
	)
	left := testPass(EyeLeft, leftCamera, geometry,
		Material{BaseColor: Vec3{1, 1, 1}}, Identity4())
	right := testPass(EyeRight, rightCamera, geometry,
		Material{BaseColor: Vec3{1, 1, 1}}, Identity4())

	shared, err := RenderShared(left, right)
	if err != nil {
		t.Fatalf("shared render: %v", err)
	}
	reference, err := RenderReconstructed(left, right)
	if err != nil {
		t.Fatalf("reference render: %v", err)
	}
	if shared.Stats.Left.ClippedTriangles != 1 {
		t.Fatalf("left clipped triangles = %d, want 1",
			shared.Stats.Left.ClippedTriangles)
	}
	if shared.Stats.Right.ClippedTriangles != 0 {
		t.Fatalf("right clipped triangles = %d, want 0 (left clipping leaked)",
			shared.Stats.Right.ClippedTriangles)
	}
	if !CompareResults(shared, reference).ExactMatch {
		t.Fatal("clipped shared render differs from reference")
	}
}

func TestEyeMatricesDoNotAlias(t *testing.T) {
	geometry := testQuad()
	left := testPass(EyeLeft, testCamera(-0.35), geometry,
		Material{BaseColor: Vec3{1, 0.5, 0.25}}, Identity4())
	right := testPass(EyeRight, testCamera(0.35), geometry,
		Material{BaseColor: Vec3{0.25, 0.5, 1}}, Identity4())

	first, err := RenderShared(left, right)
	if err != nil {
		t.Fatalf("first render: %v", err)
	}
	if first.Stats.Left.TransformDigest == first.Stats.Right.TransformDigest {
		t.Fatal("left and right transform uploads have the same fingerprint")
	}

	otherRight := right
	otherRight.Camera = testCamera(0.9)
	second, err := RenderShared(left, otherRight)
	if err != nil {
		t.Fatalf("second render: %v", err)
	}
	if !first.Left.Equal(second.Left) {
		t.Fatal("left image changed when only right camera changed")
	}
	if first.Stats.Left.TransformDigest != second.Stats.Left.TransformDigest {
		t.Fatal("left transform upload changed when only right camera changed")
	}
}

func TestInvalidGeometryRejected(t *testing.T) {
	bad := &Geometry{
		Name:     "bad",
		Vertices: []Vertex{{}},
		Indices:  []uint32{0, 1, 0},
	}
	left := testPass(EyeLeft, testCamera(0), bad,
		Material{BaseColor: Vec3{1, 1, 1}}, Identity4())
	right := testPass(EyeRight, testCamera(0), bad,
		Material{BaseColor: Vec3{1, 1, 1}}, Identity4())
	if _, err := RenderShared(left, right); err == nil {
		t.Fatal("expected invalid index error")
	}
}
