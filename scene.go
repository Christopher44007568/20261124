package depthmip

import (
	"fmt"
	"math"
)

// Vec3 is a 3D vector/point.
type Vec3 [3]float64

func (a Vec3) Sub(b Vec3) Vec3    { return Vec3{a[0] - b[0], a[1] - b[1], a[2] - b[2]} }
func (a Vec3) Dot(b Vec3) float64 { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }

// Sphere is an analytic sphere primitive.
type Sphere struct {
	C Vec3
	R float64
}

// Plane is an analytic plane N.P + D = 0.
type Plane struct {
	N Vec3
	D float64
}

// Scene is a set of analytic primitives plus the depth mapping range.
type Scene struct {
	Spheres   []Sphere
	Planes    []Plane
	Near, Far float64
}

// intersect returns the smallest positive ray parameter t, or +Inf on miss.
func (s *Scene) intersect(ro, rd Vec3) float64 {
	t := math.Inf(1)
	for _, sp := range s.Spheres {
		oc := ro.Sub(sp.C)
		b := oc.Dot(rd)
		c := oc.Dot(oc) - sp.R*sp.R
		disc := b*b - c
		if disc < 0 {
			continue
		}
		sq := math.Sqrt(disc)
		for _, cand := range [2]float64{-b - sq, -b + sq} {
			if cand > 0 && cand < t {
				t = cand
			}
		}
	}
	for _, pl := range s.Planes {
		denom := pl.N.Dot(rd)
		if denom == 0 {
			continue
		}
		cand := -(pl.N.Dot(ro) + pl.D) / denom
		if cand > 0 && cand < t {
			t = cand
		}
	}
	return t
}

// Depth casts one ray and returns the reversed-Z depth: 1 at the near
// plane, 0 at/beyond the far plane or on miss.
func (s *Scene) Depth(ro, rd Vec3) float64 {
	t := s.intersect(ro, rd)
	if math.IsInf(t, 1) || t >= s.Far {
		return 0
	}
	if t <= s.Near {
		return 1
	}
	return (s.Far - t) / (s.Far - s.Near)
}

// RenderReference renders the scene analytically through a pinhole camera at
// the origin looking down +Z, producing the ground-truth base depth buffer.
func (s *Scene) RenderReference(w, h int) (*Buffer, error) {
	if s.Near <= 0 || s.Far <= s.Near {
		return nil, fmt.Errorf("invalid depth range: near=%g far=%g", s.Near, s.Far)
	}
	buf, err := NewBuffer(w, h)
	if err != nil {
		return nil, err
	}
	const halfFovTan = 0.5 // ~53 degree vertical FOV
	aspect := float64(w) / float64(h)
	for y := 0; y < h; y++ {
		v := (1 - 2*(float64(y)+0.5)/float64(h)) * halfFovTan
		for x := 0; x < w; x++ {
			u := (2*(float64(x)+0.5)/float64(w) - 1) * halfFovTan * aspect
			rd := Vec3{u, v, 1}
			buf.Data[y*w+x] = s.Depth(Vec3{}, rd)
		}
	}
	return buf, nil
}
