package main

import "testing"

func TestSimplificationProtectsBoundaryAndReducesMesh(t *testing.T) {
	source := makeAuditMesh()
	lods := buildLODs(source)
	if len(lods) != 3 {
		t.Fatalf("LOD count = %d, want 3", len(lods))
	}
	lod := lods[2]

	if len(lod.Triangles) >= len(source.Triangles) {
		t.Fatalf("LOD triangles = %d, want fewer than %d", len(lod.Triangles), len(source.Triangles))
	}

	sourceBoundary := boundaryPoints(source)
	lodBoundary := boundaryPoints(lod)
	if len(sourceBoundary) != len(lodBoundary) {
		t.Fatalf("boundary vertex count = %d, want %d", len(lodBoundary), len(sourceBoundary))
	}

	tree := buildKDTree(lodBoundary)
	for _, point := range sourceBoundary {
		_, _, distance := tree.nearest(point)
		if distance != 0 {
			t.Fatalf("boundary moved by %f, want zero movement", distance)
		}
	}
}

func TestMissingCoveragePixelFails(t *testing.T) {
	camera := auditCameras()[0]
	reference := renderMesh(makeAuditMesh(), camera)
	candidate := renderMesh(makeAuditMesh(), camera)

	for index, covered := range reference.Covered {
		if covered {
			candidate.Covered[index] = false
			break
		}
	}

	metrics := calculateMetrics(reference, candidate, makeAuditMesh(), makeAuditMesh(), camera)
	if metrics.MissingPixels != 1 || metrics.OnePixelBreaches != 1 || metrics.Pass {
		t.Fatalf("metrics = %+v, want one missing pixel and failed audit", metrics)
	}
}

func boundaryPoints(mesh *Mesh) []Vec3 {
	var points []Vec3
	for index, boundary := range mesh.Boundary {
		if boundary {
			points = append(points, mesh.Vertices[index])
		}
	}
	return points
}
