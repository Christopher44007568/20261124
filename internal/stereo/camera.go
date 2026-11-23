package stereo

import "fmt"

// Frustum stores tangent-plane extents at one unit along the view axis.
// Asymmetric values are intentional: x = left*z .. right*z and
// y = bottom*z .. top*z. Both eyes therefore keep their own projection.
type Frustum struct {
	Left   float64 `json:"left"`
	Right  float64 `json:"right"`
	Bottom float64 `json:"bottom"`
	Top    float64 `json:"top"`
}

// Camera is a right-handed eye camera with x right, y up, and z forward.
type Camera struct {
	Name      string  `json:"name"`
	Position  Vec3    `json:"position"`
	Target    Vec3    `json:"target"`
	Right     Vec3    `json:"right"`
	Up        Vec3    `json:"up"`
	Forward   Vec3    `json:"forward"`
	Near      float64 `json:"near"`
	Far       float64 `json:"far"`
	Frustum   Frustum `json:"frustum"`
	ViewportW int     `json:"viewportWidth"`
	ViewportH int     `json:"viewportHeight"`
}

// NewCamera builds an orthonormal camera basis. The target only fixes the
// direction; the supplied tangents define the asymmetric frustum.
func NewCamera(name string, position, target, up Vec3, near, far float64, f Frustum, width, height int) (Camera, error) {
	forward := target.Sub(position).Normalized()
	if forward.Len() == 0 {
		return Camera{}, fmt.Errorf("%s: camera target equals position", name)
	}
	right := up.Cross(forward).Normalized()
	if right.Len() == 0 {
		return Camera{}, fmt.Errorf("%s: up vector is parallel to view direction", name)
	}
	trueUp := forward.Cross(right).Normalized()
	if near <= 0 || far <= near {
		return Camera{}, fmt.Errorf("%s: require 0 < near < far", name)
	}
	if f.Left >= f.Right || f.Bottom >= f.Top {
		return Camera{}, fmt.Errorf("%s: invalid frustum tangents", name)
	}
	if width <= 0 || height <= 0 {
		return Camera{}, fmt.Errorf("%s: viewport must be positive", name)
	}
	return Camera{
		Name:      name,
		Position:  position,
		Target:    target,
		Right:     right,
		Up:        trueUp,
		Forward:   forward,
		Near:      near,
		Far:       far,
		Frustum:   f,
		ViewportW: width,
		ViewportH: height,
	}, nil
}

// ToView converts a world point to this eye's view space.
func (c Camera) ToView(p Vec3) Vec3 {
	d := p.Sub(c.Position)
	return Vec3{d.Dot(c.Right), d.Dot(c.Up), d.Dot(c.Forward)}
}

// NearDistance returns the signed view-space distance to the near plane.
func (c Camera) NearDistance(p Vec3) float64 {
	return p.Sub(c.Position).Dot(c.Forward) - c.Near
}

// NearPlaneCenter is the point on this eye's near plane directly in front of
// its optical center. It is deliberately reported per eye.
func (c Camera) NearPlaneCenter() Vec3 {
	return c.Position.Add(c.Forward.Mul(c.Near))
}

// ProjectView maps a point with positive view z into normalized device
// coordinates. The caller is responsible for near clipping first.
func (c Camera) ProjectView(v Vec3) Vec2 {
	tx := v.X / v.Z
	ty := v.Y / v.Z
	f := c.Frustum
	return Vec2{
		X: (2*tx - (f.Right + f.Left)) / (f.Right - f.Left),
		Y: (2*ty - (f.Top + f.Bottom)) / (f.Top - f.Bottom),
	}
}

// NDCToPixel maps [-1,1]^2 to the viewport, with y increasing downward.
func (c Camera) NDCToPixel(p Vec2) Vec2 {
	return Vec2{
		X: (p.X + 1) * 0.5 * float64(c.ViewportW),
		Y: (1 - p.Y) * 0.5 * float64(c.ViewportH),
	}
}

// ViewToPixel projects an already near-clipped view-space point.
func (c Camera) ViewToPixel(v Vec3) Vec2 {
	return c.NDCToPixel(c.ProjectView(v))
}
