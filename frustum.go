package main

import "fmt"

// Plane is a half-space: inside iff N·p + D >= 0.
type Plane struct {
	N Vec3
	D float64
}

func (p Plane) Dist(v Vec3) float64 { return p.N.Dot(v) + p.D }

// Plane test order is fixed so "first failure" is deterministic.
const (
	PlaneNear = iota
	PlaneFar
	PlaneLeft
	PlaneRight
	PlaneBottom
	PlaneTop
	planeCount
)

var planeNames = [planeCount]string{"near", "far", "left", "right", "bottom", "top"}

// Frustum is a set of 6 clipping planes of one eye.
type Frustum struct {
	Planes [planeCount]Plane
}

// ExtractFrustum pulls the 6 planes out of a view-projection matrix
// (Gribb-Hartmann method, row-major, column-vector convention).
func ExtractFrustum(vp Mat4) Frustum {
	var f Frustum
	combo := func(r3, r int, sign float64) Plane {
		p := Plane{
			N: Vec3{vp[3][0] + sign*vp[r][0], vp[3][1] + sign*vp[r][1], vp[3][2] + sign*vp[r][2]},
			D: vp[3][3] + sign*vp[r][3],
		}
		l := p.N.Len()
		if l > 1e-12 {
			p.N = p.N.Scale(1 / l)
			p.D /= l
		}
		return p
	}
	f.Planes[PlaneLeft] = combo(3, 0, 1)
	f.Planes[PlaneRight] = combo(3, 0, -1)
	f.Planes[PlaneBottom] = combo(3, 1, 1)
	f.Planes[PlaneTop] = combo(3, 1, -1)
	f.Planes[PlaneNear] = combo(3, 2, 1)
	f.Planes[PlaneFar] = combo(3, 2, -1)
	return f
}

// EyeEvidence records why one eye accepted or rejected an instance.
type EyeEvidence struct {
	Visible     bool
	FirstFail   string   // first plane that culled the box, if any
	Crossed     []string // planes the box straddles (e.g. near-plane crossing)
	CrossesNear bool
}

func (e EyeEvidence) String() string {
	if !e.Visible {
		return fmt.Sprintf("不可见(首次失败:%s面)", e.FirstFail)
	}
	s := "可见"
	if len(e.Crossed) > 0 {
		s += fmt.Sprintf("(穿越%v面)", e.Crossed)
	}
	return s
}

// CullOBB tests a world-space OBB against a frustum, plane by plane.
// It is conservative (may keep boxes outside the frustum corner region),
// records the first failing plane and every straddled plane as evidence.
func CullOBB(f Frustum, center Vec3, axes [3]Vec3, half Vec3) EyeEvidence {
	ev := EyeEvidence{Visible: true}
	h := [3]float64{half.X, half.Y, half.Z}
	for i := 0; i < planeCount; i++ {
		p := f.Planes[i]
		r := 0.0
		for a := 0; a < 3; a++ {
			d := p.N.Dot(axes[a])
			if d < 0 {
				d = -d
			}
			r += d * h[a]
		}
		dist := p.Dist(center)
		if dist < -r {
			ev.Visible = false
			ev.FirstFail = planeNames[i]
			ev.Crossed = nil
			return ev
		}
		if dist < r {
			ev.Crossed = append(ev.Crossed, planeNames[i])
			if i == PlaneNear {
				ev.CrossesNear = true
			}
		}
	}
	return ev
}
