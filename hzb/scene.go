package hzb

// Object is a culling target identified by an AABB.
type Object struct {
	ID  string
	Box AABB
}

// Scene holds occluder geometry and the objects being culled.
type Scene struct {
	Occluders []Triangle
	Targets   []Object
}

// BoxTriangles returns the 12 triangles of an axis-aligned box.
func BoxTriangles(min, max Vec3) []Triangle {
	c := [8]Vec3{
		{min.X, min.Y, min.Z}, {max.X, min.Y, min.Z},
		{min.X, max.Y, min.Z}, {max.X, max.Y, min.Z},
		{min.X, min.Y, max.Z}, {max.X, min.Y, max.Z},
		{min.X, max.Y, max.Z}, {max.X, max.Y, max.Z},
	}
	quad := func(a, b, cIdx, d int) []Triangle {
		return []Triangle{
			{c[a], c[b], c[cIdx]},
			{c[a], c[cIdx], c[d]},
		}
	}
	var tris []Triangle
	tris = append(tris, quad(0, 1, 3, 2)...) // -Z
	tris = append(tris, quad(4, 6, 7, 5)...) // +Z
	tris = append(tris, quad(0, 4, 5, 1)...) // -Y
	tris = append(tris, quad(2, 3, 7, 6)...) // +Y
	tris = append(tris, quad(0, 2, 6, 4)...) // -X
	tris = append(tris, quad(1, 5, 7, 3)...) // +X
	return tris
}

// VisiblePixels rasterizes the box and counts pixels whose depth beats the
// current depth buffer, i.e. pixels that would actually be seen.
func VisiblePixels(cam *Camera, db *DepthBuffer, box AABB) int {
	count := 0
	rasterize(cam, BoxTriangles(box.Min, box.Max), func(x, y int, z float64) bool {
		if z+OcclusionEpsilon < db.Depth[y*db.W+x] {
			count++
		}
		return false
	})
	return count
}

// GroundTruthVisible reports whether any pixel of the box survives a depth
// test against the eye's full-resolution depth buffer.
func GroundTruthVisible(cam *Camera, db *DepthBuffer, box AABB) bool {
	return VisiblePixels(cam, db, box) > 0
}
