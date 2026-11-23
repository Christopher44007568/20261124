package binocular

import (
	"fmt"
	"math"
)

// Rasterizer is a stateless software renderer. All mutable frame state lives in
// renderEye, making it safe to reuse one Rasterizer for many frames.
type Rasterizer struct{}

// NewRasterizer creates a software rasterizer.
func NewRasterizer() *Rasterizer {
	return &Rasterizer{}
}

// RenderShared renders both eyes directly from the Geometry values in the two
// passes. Matching pointers are uploaded once, while every eye still supplies
// its own Camera, instance transform, and Material.
func (r *Rasterizer) RenderShared(left, right EyePass) (Result, error) {
	return r.renderPair(left, right, ModeShared)
}

// RenderReconstructed is the reference path. It deep-copies each eye's geometry
// payload before rendering, emulating two independent reconstruction passes.
func (r *Rasterizer) RenderReconstructed(left, right EyePass) (Result, error) {
	return r.renderPair(left, right, ModeReconstructed)
}

// RenderShared is a convenience wrapper around a zero-value Rasterizer.
func RenderShared(left, right EyePass) (Result, error) {
	return NewRasterizer().RenderShared(left, right)
}

// RenderReconstructed is a convenience wrapper around a zero-value Rasterizer.
func RenderReconstructed(left, right EyePass) (Result, error) {
	return NewRasterizer().RenderReconstructed(left, right)
}

func (r *Rasterizer) renderPair(left, right EyePass, mode RenderMode) (Result, error) {
	if err := left.validate(); err != nil {
		return Result{}, err
	}
	if err := right.validate(); err != nil {
		return Result{}, err
	}

	stats := StereoStats{Mode: mode}
	copies := 0
	if mode == ModeReconstructed {
		left = clonePass(left)
		right = clonePass(right)
		copies = countUniqueGeometries(left) + countUniqueGeometries(right)
	}

	stats.SharedVertexCount, stats.SharedIndexCount = countSharedPayload(left, right)
	stats.GeometryUploads, stats.GeometryUploadBytes =
		countGeometryUploads(left, right)
	stats.ReconstructedGeometryCopies = copies

	leftImage, leftStats := renderEye(left)
	rightImage, rightStats := renderEye(right)
	if leftImage.Width != rightImage.Width || leftImage.Height != rightImage.Height {
		return Result{}, fmt.Errorf("binocular: eye target sizes differ: %dx%d vs %dx%d",
			leftImage.Width, leftImage.Height, rightImage.Width, rightImage.Height)
	}

	stats.Left = leftStats
	stats.Right = rightStats
	stats.Batches = leftStats.Batches + rightStats.Batches
	stats.VertexReads = leftStats.VertexReads + rightStats.VertexReads
	stats.IndexReads = leftStats.IndexReads + rightStats.IndexReads
	stats.TransformUploads = leftStats.TransformUploads + rightStats.TransformUploads
	stats.TransformUploadBytes =
		leftStats.TransformUploadBytes + rightStats.TransformUploadBytes
	stats.MaterialUploads = leftStats.MaterialUploads + rightStats.MaterialUploads
	stats.MaterialUploadBytes =
		leftStats.MaterialUploadBytes + rightStats.MaterialUploadBytes
	if mode == ModeShared {
		stats.SharedGeometryAccesses =
			leftStats.GeometryAccesses + rightStats.GeometryAccesses
	}

	return Result{
		Mode:  mode,
		Left:  leftImage,
		Right: rightImage,
		Stats: stats,
	}, nil
}

func countUniqueGeometries(pass EyePass) int {
	seen := make(map[*Geometry]struct{})
	for _, item := range pass.Items {
		seen[item.Geometry] = struct{}{}
	}
	return len(seen)
}

func countSharedPayload(left, right EyePass) (vertices, indices int) {
	seen := make(map[*Geometry]struct{})
	for _, pass := range []EyePass{left, right} {
		for _, item := range pass.Items {
			if _, ok := seen[item.Geometry]; ok {
				continue
			}
			seen[item.Geometry] = struct{}{}
			vertices += len(item.Geometry.Vertices)
			indices += len(item.Geometry.Indices)
		}
	}
	return vertices, indices
}

func countGeometryUploads(left, right EyePass) (uploads int, bytes int64) {
	seen := make(map[*Geometry]struct{})
	for _, pass := range []EyePass{left, right} {
		for _, item := range pass.Items {
			if _, ok := seen[item.Geometry]; ok {
				continue
			}
			seen[item.Geometry] = struct{}{}
			uploads++
			bytes += item.Geometry.UploadBytes()
		}
	}
	return uploads, bytes
}

func renderEye(pass EyePass) (Image, EyeStats) {
	camera := pass.Camera
	stats := newEyeStats(pass.Eye)
	image := NewImage(camera.Width, camera.Height, camera.Clear)
	depth := make([]float64, camera.Width*camera.Height)
	for i := range depth {
		depth[i] = math.Inf(1)
	}

	// These are local copies. Nothing in this function reads camera matrices
	// from the other eye, even when both passes point at the same Geometry.
	view := camera.View
	projection := camera.Projection
	for _, item := range pass.Items {
		stats.Batches++
		stats.GeometryAccesses++
		model := item.Model
		stats.noteTransform(model, view, projection)
		stats.noteMaterial(item.Material)

		modelView := view.Mul(model)
		modelViewProjection := projection.Mul(modelView)
		for triangle := 0; triangle < len(item.Geometry.Indices); triangle += 3 {
			stats.Triangles++
			vertices, indices := loadTriangle(item.Geometry, triangle)
			stats.VertexReads += int64(len(vertices))
			stats.IndexReads += int64(len(indices))
			drawTriangle(&image, depth, &stats, modelViewProjection,
				vertices, item.Material)
		}
	}
	return image, stats
}

func loadTriangle(geometry *Geometry, firstIndex int) ([3]Vertex, [3]uint32) {
	indices := [3]uint32{
		geometry.Indices[firstIndex],
		geometry.Indices[firstIndex+1],
		geometry.Indices[firstIndex+2],
	}
	vertices := [3]Vertex{
		geometry.Vertices[indices[0]],
		geometry.Vertices[indices[1]],
		geometry.Vertices[indices[2]],
	}
	return vertices, indices
}
