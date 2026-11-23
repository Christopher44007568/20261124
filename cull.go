package main

import "math"

// FrameMetrics records the four required counters for one frame.
type FrameMetrics struct {
	Frame                int  `json:"frame"`
	SharedTraversal      int  `json:"shared_traversal"`      // object x occluder tests in shared layer
	ComplementTraversal  int  `json:"complement_traversal"`  // tests in per-eye complement layer
	ConservativeFallback int  `json:"conservative_fallback"` // range expansions after per-eye occluder failure
	FalseCulled          int  `json:"false_culled"`          // objects culled but actually visible
	SharedFailure        bool `json:"shared_failure"`        // shared set culled an object visible to an eye
	VisibleLeft          int  `json:"visible_left"`
	VisibleRight         int  `json:"visible_right"`
}

// coverage of an object sphere by one occluder from one eye.
type coverage int

const (
	coverNone coverage = iota
	coverPartial
	coverFull
)

// silhouetteDirs returns sample directions from the eye that cover the
// object's angular footprint: the center plus 8 tangent-ring points.
func silhouetteDirs(eye Vec3, obj Object, radiusScale float64) []Vec3 {
	toC := Sub(obj.Center, eye)
	d := Len(toC)
	if d < 1e-9 {
		return []Vec3{{0, 0, 1}}
	}
	fwd := Mul(toC, 1/d)
	r := obj.Radius * radiusScale
	if r >= d {
		r = d * 0.99
	}
	// Build a tangent ring at the sphere silhouette.
	sinA := r / d
	cosA := math.Sqrt(1 - sinA*sinA)
	up := Vec3{0, 1, 0}
	if math.Abs(Dot(fwd, up)) > 0.95 {
		up = Vec3{1, 0, 0}
	}
	u := Norm(Cross(fwd, up))
	v := Cross(fwd, u)
	dirs := make([]Vec3, 0, 9)
	dirs = append(dirs, fwd)
	for i := 0; i < 8; i++ {
		a := 2 * math.Pi * float64(i) / 8
		off := Add(Mul(u, math.Cos(a)*sinA), Mul(v, math.Sin(a)*sinA))
		dirs = append(dirs, Norm(Add(Mul(fwd, cosA), off)))
	}
	return dirs
}

// testOcclusion checks how completely one occluder covers an object from eye.
// It counts as one traversal unit.
func testOcclusion(eye Vec3, obj Object, occ Occluder, radiusScale float64) coverage {
	hit, miss := 0, 0
	for _, dir := range silhouetteDirs(eye, obj, radiusScale) {
		ray := Ray{Orig: eye, Dir: dir}
		tSphere := raySphere(ray, obj.Center, obj.Radius*radiusScale)
		tBox := rayBox(ray, occ.Min, occ.Max)
		if tBox > 0 && (tSphere < 0 || tBox < tSphere) {
			hit++
		} else {
			miss++
		}
	}
	switch {
	case miss == 0:
		return coverFull
	case hit == 0:
		return coverNone
	default:
		return coverPartial
	}
}

// occludedFromEye reports whether the union of occluders fully covers the
// object from eye, counting traversal units into counter.
func occludedFromEye(eye Vec3, obj Object, occs []Occluder, counter *int) bool {
	dirs := silhouetteDirs(eye, obj, 1.0)
	blocked := make([]bool, len(dirs))
	for _, occ := range occs {
		*counter++
		all := true
		for i, dir := range dirs {
			if blocked[i] {
				continue
			}
			all = false
			ray := Ray{Orig: eye, Dir: dir}
			tSphere := raySphere(ray, obj.Center, obj.Radius)
			tBox := rayBox(ray, occ.Min, occ.Max)
			if tBox > 0 && (tSphere < 0 || tBox < tSphere) {
				blocked[i] = true
			}
		}
		if all {
			break
		}
	}
	for _, b := range blocked {
		if !b {
			return false
		}
	}
	return true
}
