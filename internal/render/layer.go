package render

import (
	"math"

	"curvelayers/internal/geom"
	"curvelayers/internal/tex"
)

// Layer binds an analytical surface to its marker placement and body tint.
type Layer struct {
	Surface   *geom.Surface
	Placement tex.Placement
	Base      [3]float64
	Kind      int
}

// paint evaluates the surface material at one inverse-mapped parameter point.
// The graticule is drawn in parameter space, so the sphere's meridians visibly
// bunch toward the poles (non-uniform scaling) without any planar warp.
func paintLayer(l Layer, hit geom.Hit) (rgb [3]float64, alpha float64) {
	rgb = l.Base
	const divisions = 24 // grid lines every 15 degrees of longitude/latitude
	gridU := gridCoverage(hit.U * divisions)
	gridV := gridCoverage(hit.V * divisions)
	grid := math.Max(gridU, gridV)
	rgb[0] = rgb[0]*(1-grid*0.55) + 0.75*grid*0.55
	rgb[1] = rgb[1]*(1-grid*0.55) + 0.78*grid*0.55
	rgb[2] = rgb[2]*(1-grid*0.55) + 0.82*grid*0.55
	alpha = 1
	return rgb, alpha
}

func gridCoverage(phase float64) float64 {
	d := math.Abs(phase - math.Round(phase))
	c := 0.18 - d // ~1.5% parameter width with a short AA ramp
	if c <= 0 {
		return 0
	}
	if c > 1 {
		return 1
	}
	return c
}

// DefaultScene constructs the plane, cylinder and equirectangular sphere with
// overlapping footprints so depth order actually changes across the frame.
func DefaultScene() ([]Layer, []*geom.Surface) {
	plane := &geom.Surface{ID: 0, Kind: geom.KindPlane, Center: geom.Vec3{Z: 6.6}, HalfW: 2.6, HalfH: 1.95}
	cylinder := &geom.Surface{ID: 1, Kind: geom.KindCylinder, Center: geom.Vec3{Z: 5.7}, Radius: 2.4, HalfH: 1.95}
	sphere := &geom.Surface{ID: 2, Kind: geom.KindSphere, Center: geom.Vec3{Z: 4.7}, Radius: 1.25}

	layers := []Layer{
		{
			Surface: plane,
			Base:    [3]float64{0.16, 0.18, 0.24},
			Kind:    geom.KindPlane,
			Placement: tex.Placement{
				CenterU: 0.5, CenterV: 0.5, HalfU: 0.16, HalfV: 0.27,
				WrapU: tex.WrapClamp, WrapV: tex.WrapClamp,
			},
		},
		{
			Surface: cylinder,
			Base:    [3]float64{0.13, 0.22, 0.18},
			Kind:    geom.KindCylinder,
			Placement: tex.Placement{
				// Marker straddles the U=0/1 seam facing the camera.
				CenterU: 0, CenterV: 0.5, HalfU: 0.11, HalfV: 0.24,
				WrapU: tex.WrapRepeat, WrapV: tex.WrapClamp,
			},
		},
		{
			Surface: sphere,
			Base:    [3]float64{0.24, 0.16, 0.22},
			Kind:    geom.KindSphere,
			Placement: tex.Placement{
				// Marker straddles the front longitude seam just below the
				// north pole; longitude repeats, latitude paints the edge.
				CenterU: 0.5, CenterV: 0.78, HalfU: 0.13, HalfV: 0.13,
				WrapU: tex.WrapRepeat, WrapV: tex.WrapClampPaint,
			},
		},
	}
	surfaces := []*geom.Surface{plane, cylinder, sphere}
	return layers, surfaces
}
