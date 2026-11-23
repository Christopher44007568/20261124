package geom

import "math"

// Camera is a pinhole camera looking down +Z. Positive Yaw turns the view
// toward +X, positive Pitch lifts the view toward +Y.
type Camera struct {
	Pos           Vec3
	Yaw, Pitch    float64
	Near, Far     float64
	FovY          float64
	Width, Height int
}

// Forward is the view direction at the image center.
func (c Camera) Forward() Vec3 {
	cy, sy := math.Cos(c.Yaw), math.Sin(c.Yaw)
	cp, sp := math.Cos(c.Pitch), math.Sin(c.Pitch)
	return V(sy*cp, sp, cy*cp).Normalize()
}

// Right is the normalized image-right axis in world space.
func (c Camera) Right() Vec3 {
	return c.Forward().Cross(V(0, 1, 0)).Normalize()
}

// Up is the normalized image-up axis in world space.
func (c Camera) Up() Vec3 {
	return c.Right().Cross(c.Forward()).Normalize()
}

// Aspect returns width / height.
func (c Camera) Aspect() float64 { return float64(c.Width) / float64(c.Height) }

func (c Camera) tanHalfFov() float64 { return math.Tan(c.FovY * 0.5) }

// Project converts a world point to pixel coordinates plus positive view-space
// linear depth. Inside is false when the point lies behind the near plane or
// outside the view frustum.
func (c Camera) Project(p Vec3) (u, v, depth float64, inside bool) {
	rel := p.Sub(c.Pos)
	f, r, up := c.Forward(), c.Right(), c.Up()
	z := rel.Dot(f)
	if z <= c.Near || z >= c.Far {
		return 0, 0, 0, false
	}
	x := rel.Dot(r)
	y := rel.Dot(up)
	t := c.tanHalfFov()
	ndcX := x / (z * t * c.Aspect())
	ndcY := y / (z * t)
	if ndcX < -1 || ndcX > 1 || ndcY < -1 || ndcY > 1 {
		return 0, 0, z, false
	}
	u = (ndcX + 1) * 0.5 * float64(c.Width)
	v = (1 - ndcY) * 0.5 * float64(c.Height)
	return u, v, z, true
}

// Unproject turns a pixel center and a positive linear depth back into a
// world point. It is the exact inverse of Project for that depth.
func (c Camera) Unproject(px, py, depth float64) Vec3 {
	f, r, up := c.Forward(), c.Right(), c.Up()
	t := c.tanHalfFov()
	ndcX := (px+0.5)/float64(c.Width)*2 - 1
	ndcY := 1 - ((py + 0.5) / float64(c.Height) * 2)
	x := ndcX * depth * t * c.Aspect()
	y := ndcY * depth * t
	return c.Pos.Add(r.Scale(x)).Add(up.Scale(y)).Add(f.Scale(depth))
}
