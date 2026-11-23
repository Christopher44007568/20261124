package stereocull

import "math"

// CullResult classifies a bounded volume against a frustum.
type CullResult int

const (
	CullOutside    CullResult = iota // fully outside, safe to reject
	CullIntersects                   // crosses a frustum plane
	CullInside                       // fully inside every plane
)

// Frustum is the intersection of six inward-facing half-spaces
// (near, far, left, right, top, bottom).
type Frustum struct {
	Planes [6]Plane
}

// CameraSpec describes one eye in head-local space. The head looks down -Z
// with +Y up and +X to the left; the eye sits at (EyeX, 0, 0).
type CameraSpec struct {
	EyeX      float64 // +IPD/2 for the left eye, -IPD/2 for the right
	Near, Far float64
	HalfW     float64 // half width of the near rectangle
	HalfH     float64 // half height of the near rectangle
}

// BuildFrustum constructs the six frustum planes in head-local space.
func BuildFrustum(c CameraSpec) Frustum {
	n := c.Near
	hn := c.HalfH
	wn := c.HalfW
	lv := math.Sqrt(n*n + hn*hn)
	lh := math.Sqrt(n*n + wn*wn)
	nearPoint := Vec3{Z: -n}
	farPoint := Vec3{Z: -c.Far}

	top := PlaneFromPoint(Vec3{Y: -n / lv, Z: -hn / lv}, Vec3{X: c.EyeX, Y: hn, Z: -n})
	bottom := PlaneFromPoint(Vec3{Y: n / lv, Z: -hn / lv}, Vec3{X: c.EyeX, Y: -hn, Z: -n})
	left := PlaneFromPoint(Vec3{X: -n / lh, Z: -wn / lh}, Vec3{X: c.EyeX + wn, Z: -n})
	right := PlaneFromPoint(Vec3{X: n / lh, Z: -wn / lh}, Vec3{X: c.EyeX - wn, Z: -n})

	near := PlaneFromPoint(Vec3{Z: -1}, nearPoint)
	far := PlaneFromPoint(Vec3{Z: 1}, farPoint)

	return Frustum{Planes: [6]Plane{near, far, left, right, top, bottom}}
}

// BuildUnionFrustum returns a frustum that contains both eye frusta. For
// parallel stereo it is their exact convex hull: shared near/far/top/bottom
// planes plus the outer side plane of each eye. Objects rejected by it are
// rejected by both eyes; objects accepted by it still need per-eye tests.
func BuildUnionFrustum(left, right CameraSpec) Frustum {
	lf := BuildFrustum(left)
	rf := BuildFrustum(right)
	// Plane order: near, far, left(outer=left eye's left), right(outer=right
	// eye's right), top, bottom.
	return Frustum{Planes: [6]Plane{
		lf.Planes[0],
		lf.Planes[1],
		lf.Planes[2],
		rf.Planes[3],
		lf.Planes[4],
		lf.Planes[5],
	}}
}

// Transform moves the frustum from head-local space into world space.
func (f Frustum) Transform(r Quat, t Vec3) Frustum {
	out := Frustum{}
	for i, p := range f.Planes {
		out.Planes[i] = p.Transform(r, t)
	}
	return out
}

// TestSphere classifies a bounding sphere.
func (f Frustum) TestSphere(center Vec3, radius float64) CullResult {
	status := CullInside
	for _, p := range f.Planes {
		d := p.Distance(center)
		if d < -radius {
			return CullOutside
		}
		if d < radius {
			status = CullIntersects
		}
	}
	return status
}

// AABB is an axis-aligned bounding box.
type AABB struct {
	Min, Max Vec3
}

// TestAABB classifies a box with the p-vertex (positive vertex) test.
func (f Frustum) TestAABB(b AABB) CullResult {
	status := CullInside
	for _, p := range f.Planes {
		nx, ny, nz := p.Normal.X, p.Normal.Y, p.Normal.Z
		farX, nearX := b.Max.X, b.Min.X
		if nx < 0 {
			farX, nearX = b.Min.X, b.Max.X
		}
		farY, nearY := b.Max.Y, b.Min.Y
		if ny < 0 {
			farY, nearY = b.Min.Y, b.Max.Y
		}
		farZ, nearZ := b.Max.Z, b.Min.Z
		if nz < 0 {
			farZ, nearZ = b.Min.Z, b.Max.Z
		}
		if nx*farX+ny*farY+nz*farZ+p.D < 0 {
			return CullOutside
		}
		if nx*nearX+ny*nearY+nz*nearZ+p.D < 0 {
			status = CullIntersects
		}
	}
	return status
}
