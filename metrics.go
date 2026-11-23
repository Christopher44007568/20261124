package main

import (
	"math"
	"sort"
)

type ErrorStats struct {
	Max  float64 `json:"max"`
	P95  float64 `json:"p95"`
	RMS  float64 `json:"rms"`
	Mean float64 `json:"mean"`
}

type MetricSet struct {
	ScreenSpaceError ErrorStats `json:"screenSpaceErrorPx"`
	NormalJump       ErrorStats `json:"normalJumpDegrees"`
	BoundaryCrack    ErrorStats `json:"boundaryCrackPx"`
	MissingPixels    int        `json:"missingPixels"`
	ExtraPixels      int        `json:"extraPixels"`
	OnePixelBreaches int        `json:"onePixelBreaches"`
	Pass             bool       `json:"pass"`
}

func summarize(values []float64) ErrorStats {
	if len(values) == 0 {
		return ErrorStats{}
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)

	var sum, squares float64
	for _, value := range values {
		sum += value
		squares += value * value
	}
	if math.IsNaN(sum) || math.IsNaN(squares) {
		return ErrorStats{}
	}
	index := int(math.Ceil(0.95*float64(len(values)))) - 1
	if index < 0 {
		index = 0
	}
	return ErrorStats{
		Max:  sorted[len(sorted)-1],
		P95:  sorted[index],
		RMS:  math.Sqrt(squares / float64(len(values))),
		Mean: sum / float64(len(values)),
	}
}

func calculateMetrics(reference, candidate RenderBuffer, referenceMesh, candidateMesh *Mesh, camera Camera) MetricSet {
	missingPixels := 0
	extraPixels := 0
	missingMask := make([]bool, len(reference.Covered))
	extraMask := make([]bool, len(reference.Covered))
	for index := range reference.Covered {
		referenceCovered := reference.Covered[index]
		candidateCovered := candidate.Covered[index]
		if referenceCovered && !candidateCovered {
			missingPixels++
			missingMask[index] = true
		}
		if !referenceCovered && candidateCovered {
			extraPixels++
			extraMask[index] = true
		}
	}

	silhouetteDistances := make([]float64, 0, missingPixels+extraPixels)
	for index := range missingMask {
		if missingMask[index] || extraMask[index] {
			silhouetteDistances = append(silhouetteDistances, 1)
		}
	}
	normalAngles := make([]float64, 0)
	for index := range reference.Covered {
		if !reference.Covered[index] || !candidate.Covered[index] {
			continue
		}
		cosine := reference.Normals[index].Dot(candidate.Normals[index])
		cosine = math.Max(-1, math.Min(1, cosine))
		normalAngles = append(normalAngles, math.Acos(cosine)*180/math.Pi)
	}
	basis := cameraBasisFrom(camera)
	geometryDistances := symmetricGeometryError(referenceMesh, candidateMesh, basis, camera.Width, camera.Height)
	boundaryDistances := boundaryCrackError(referenceMesh, candidateMesh, basis, camera.Width, camera.Height)
	boundaryStats := summarize(boundaryDistances)

	screenStats := summarize(geometryDistances)
	if len(silhouetteDistances) > 0 {
		screenStats.Max = math.Max(screenStats.Max, 1)
		screenStats.P95 = math.Max(screenStats.P95, 1)
	}
	return MetricSet{
		ScreenSpaceError: screenStats,
		NormalJump:       summarize(normalAngles),
		BoundaryCrack:    summarize(boundaryDistances),
		MissingPixels:    missingPixels,
		ExtraPixels:      extraPixels,
		OnePixelBreaches: missingPixels,
		Pass:             missingPixels == 0 && boundaryStats.Max < 1,
	}
}

func symmetricGeometryError(a, b *Mesh, basis cameraBasis, width, height int) []float64 {
	treeB := buildKDTree(b.Vertices)
	treeA := buildKDTree(a.Vertices)
	values := make([]float64, 0, len(a.Vertices)+len(b.Vertices))
	horizontalPixels := float64(width) * basis.zoom * 0.5
	verticalPixels := float64(height) * basis.zoom * 0.5

	projectedError := func(point, nearest Vec3) float64 {
		delta := point.Sub(nearest)
		x := math.Abs(delta.Dot(basis.right)) * horizontalPixels
		y := math.Abs(delta.Dot(basis.up)) * verticalPixels
		return math.Hypot(x, y)
	}

	for _, vertex := range a.Vertices {
		nearest, _, _ := treeB.nearest(vertex)
		values = append(values, projectedError(vertex, nearest))
	}
	for _, vertex := range b.Vertices {
		nearest, _, _ := treeA.nearest(vertex)
		values = append(values, projectedError(vertex, nearest))
	}
	return values
}

func boundaryCrackError(a, b *Mesh, basis cameraBasis, width, height int) []float64 {
	var boundaryA, boundaryB []Vec3
	for index, boundary := range a.Boundary {
		if boundary {
			boundaryA = append(boundaryA, a.Vertices[index])
		}
	}
	for index, boundary := range b.Boundary {
		if boundary {
			boundaryB = append(boundaryB, b.Vertices[index])
		}
	}

	treeB := buildKDTree(boundaryB)
	treeA := buildKDTree(boundaryA)
	values := make([]float64, 0, len(boundaryA)+len(boundaryB))
	horizontalPixels := float64(width) * basis.zoom * 0.5
	verticalPixels := float64(height) * basis.zoom * 0.5
	projectedError := func(point, nearest Vec3) float64 {
		delta := point.Sub(nearest)
		x := math.Abs(delta.Dot(basis.right)) * horizontalPixels
		y := math.Abs(delta.Dot(basis.up)) * verticalPixels
		return math.Hypot(x, y)
	}
	for _, vertex := range boundaryA {
		nearest, _, _ := treeB.nearest(vertex)
		values = append(values, projectedError(vertex, nearest))
	}
	for _, vertex := range boundaryB {
		nearest, _, _ := treeA.nearest(vertex)
		values = append(values, projectedError(vertex, nearest))
	}
	return values
}
