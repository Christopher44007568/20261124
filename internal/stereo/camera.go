package stereo

import "math"

const (
	ImageWidth  = 480
	ImageHeight = 360
	FocalPixels = 520.0
	PrincipalX  = float64(ImageWidth) / 2
	PrincipalY  = float64(ImageHeight) / 2
	MinSceneZ   = 600.0
	BrokenRefZ  = 1000.0
)

type Eye int

const (
	LeftEye  Eye = -1
	RightEye Eye = 1
)

func (e Eye) Name() string {
	if e == LeftEye {
		return "left"
	}
	return "right"
}

type Variant int

const (
	Synchronized Variant = iota
	ViewportOnly
)

func (v Variant) Name() string {
	if v == ViewportOnly {
		return "viewport-only"
	}
	return "synchronized"
}

type Ray struct {
	Origin    Vec3
	Direction Vec3
}

type Camera struct {
	IPD     float64
	Eye     Eye
	Variant Variant
}

func NewCamera(ipd float64, eye Eye, variant Variant) Camera {
	return Camera{IPD: ipd, Eye: eye, Variant: variant}
}

func (c Camera) HalfIPD() float64 {
	return c.IPD / 2
}

// EyeCenter is the physical nodal point. In the deliberately broken variant,
// both eyes keep the same view matrix at the origin.
func (c Camera) EyeCenter() Vec3 {
	if c.Variant == ViewportOnly {
		return Vec3{}
	}
	return Vec3{X: float64(c.Eye) * c.HalfIPD()}
}

// ViewportOffset converts a requested IPD into a horizontal projection-plane
// shift without moving the eye. It is the failure mode this experiment tests.
func (c Camera) ViewportOffset() float64 {
	if c.Variant != ViewportOnly {
		return 0
	}
	shift := FocalPixels * c.HalfIPD() / BrokenRefZ
	return -float64(c.Eye) * shift
}

// ProjectionOffsetAt is the horizontal projection term contributed by the
// eye translation at depth z. The synchronized form is depth dependent; the
// failed form substitutes the same viewport translation at every depth.
func (c Camera) ProjectionOffsetAt(z float64) float64 {
	if z <= 0 {
		return 0
	}
	return -FocalPixels*c.EyeCenter().X/z + c.ViewportOffset()
}

// Project returns continuous image coordinates. The perspective divide is
// shared, but the physical eye translation and the projection offset are
// updated together only in the synchronized camera.
func (c Camera) Project(p Vec3) (u, v float64, ok bool) {
	if p.Z <= 0 {
		return 0, 0, false
	}
	center := c.EyeCenter()
	u = PrincipalX + FocalPixels*(p.X-center.X)/p.Z + c.ViewportOffset()
	v = PrincipalY - FocalPixels*(p.Y-center.Y)/p.Z
	return u, v, true
}

// RayAt constructs a primary ray for continuous image coordinates. The bool is
// true when the viewport-only camera asks for a sample outside the image. That
// missing data is rendered as magenta and counted in diagnostics.
func (c Camera) RayAt(u, v float64) (Ray, bool, bool) {
	center := c.EyeCenter()
	sampleU := u
	outOfBounds := false
	if c.Variant == ViewportOnly {
		sampleU -= c.ViewportOffset()
		outOfBounds = sampleU < 0.5 || sampleU > float64(ImageWidth)-0.5
	}
	direction := Vec3{
		X: (sampleU - PrincipalX) / FocalPixels,
		Y: -(v - PrincipalY) / FocalPixels,
		Z: 1,
	}
	return Ray{Origin: center, Direction: direction}, outOfBounds, true
}

// PrincipalRay is the ray through the optical centre of the viewport.
func (c Camera) PrincipalRay() Ray {
	r, _, _ := c.RayAt(PrincipalX, PrincipalY)
	return r
}

// CenterRayOriginError reports how far the principal ray starts from the
// physical eye implied by the requested IPD.
func (c Camera) CenterRayOriginError() float64 {
	expected := Vec3{X: float64(c.Eye) * c.HalfIPD()}
	return c.PrincipalRay().Origin.Sub(expected).Length()
}

func (c Camera) ProjectedDisparity(p Vec3) (float64, bool) {
	lu, _, lok := NewCamera(c.IPD, LeftEye, c.Variant).Project(p)
	ru, _, rok := NewCamera(c.IPD, RightEye, c.Variant).Project(p)
	if !lok || !rok {
		return 0, false
	}
	return lu - ru, true
}

func DepthFromDisparity(disparity, ipd float64) float64 {
	if disparity <= 1e-9 {
		return math.Inf(1)
	}
	return FocalPixels * ipd / disparity
}

func DisparityFromDepth(depth, ipd float64) float64 {
	if depth <= 1e-9 {
		return math.Inf(1)
	}
	return FocalPixels * ipd / depth
}
