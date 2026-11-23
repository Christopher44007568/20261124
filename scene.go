package reprojlab

import (
	"fmt"
	"math"
)

type SceneObject struct {
	Name       string
	X0, Y0     int
	X1, Y1     int
	Depth      float64
	Base, Alt  RGBA
	StripeSize int
}

type Probe struct {
	Name  string
	Point Point
	Note  string
}

type Scene struct {
	Name       string
	Raster     *Raster
	Objects    []SceneObject
	Probes     []Probe
	Intrinsics Intrinsics
}

type Config struct {
	Width, Height int
	Intrinsics    Intrinsics
	Distortion    Distortion
	Translation   Translation
	LowResScale   int
}

func DefaultConfig() Config {
	return Config{
		Width:  480,
		Height: 360,
		Intrinsics: Intrinsics{
			FX: 420,
			FY: 420,
			CX: 239.5,
			CY: 179.5,
		},
		// Strong barrel curvature at the corners, plus asymmetric tangential
		// terms so horizontal and vertical order errors cannot cancel.
		Distortion: Distortion{
			K1: -0.30,
			K2: 0.24,
			K3: -0.10,
			P1: 0.020,
			P2: -0.018,
		},
		Translation: Translation{X: 0.16, Y: 0.06, Z: -0.04},
		LowResScale: 4,
	}
}

func NewScene(cfg Config) *Scene {
	s := &Scene{
		Name:       "edge-depth-scene",
		Raster:     NewRaster(cfg.Width, cfg.Height, 1, 1),
		Intrinsics: cfg.Intrinsics,
		Objects: []SceneObject{
			{Name: "left-near", X0: 0, Y0: 112, X1: 61, Y1: 258, Depth: 0.72, Base: RGBA{224, 92, 38, 255}, Alt: RGBA{255, 219, 128, 255}, StripeSize: 7},
			{Name: "right-far", X0: 419, Y0: 48, X1: 479, Y1: 196, Depth: 1.04, Base: RGBA{26, 133, 150, 255}, Alt: RGBA{196, 248, 238, 255}, StripeSize: 9},
			{Name: "bottom-mid", X0: 164, Y0: 301, X1: 321, Y1: 359, Depth: 0.88, Base: RGBA{164, 55, 145, 255}, Alt: RGBA{250, 205, 105, 255}, StripeSize: 6},
		},
		Probes: []Probe{
			{Name: "left-edge-center", Point: Point{X: 8, Y: 184}, Note: "near object touching left screen edge"},
			{Name: "left-depth-step", Point: Point{X: 60, Y: 186}, Note: "one pixel inside the near-object depth step"},
			{Name: "right-edge-center", Point: Point{X: 470, Y: 121}, Note: "far object touching right screen edge"},
			{Name: "right-depth-step", Point: Point{X: 419, Y: 123}, Note: "far-object boundary at right edge"},
			{Name: "bottom-edge-center", Point: Point{X: 242, Y: 351}, Note: "object touching bottom screen edge"},
			{Name: "background-corner", Point: Point{X: 18, Y: 24}, Note: "high-distortion background corner"},
		},
	}
	for j := 0; j < cfg.Height; j++ {
		for i := 0; i < cfg.Width; i++ {
			c, z := sceneSample(i, j, s.Objects)
			n := j*cfg.Width + i
			s.Raster.Color[n] = c
			s.Raster.Depth[n] = z
			s.Raster.Filled[n] = true
		}
	}
	return s
}

func sceneSample(x, y int, objects []SceneObject) (RGBA, float64) {
	for _, o := range objects {
		if x >= o.X0 && x <= o.X1 && y >= o.Y0 && y <= o.Y1 {
			stripe := ((x-o.X0)/o.StripeSize + (y-o.Y0)/o.StripeSize) & 1
			if stripe == 0 {
				return o.Base, o.Depth
			}
			return o.Alt, o.Depth
		}
	}
	checker := ((x/24)+(y/24))&1 == 0
	base := RGBA{54, 62, 74, 255}
	alt := RGBA{103, 111, 123, 255}
	if !checker {
		base, alt = alt, base
	}
	// A slight depth ramp exercises non-constant depth during translation.
	z := 2.2 + 0.00045*float64(x-240) - 0.00030*float64(y-180)
	return base, z
}

func (s *Scene) ProbeByName(name string) (Probe, bool) {
	for _, p := range s.Probes {
		if p.Name == name {
			return p, true
		}
	}
	return Probe{}, false
}

// DirectReference composes reprojection and distortion in one continuous map
// before the only rasterization pass.
func (s *Scene) DirectReference(cfg Config) (*Raster, WarpStats) {
	return WarpNearest(s.Raster, cfg.Width, cfg.Height, 1, 1, func(in Sample) (Point, float64, bool) {
		p, z, ok := Reproject(Point{X: in.X, Y: in.Y}, in.Depth, cfg.Intrinsics, cfg.Translation)
		if !ok {
			return Point{}, 0, false
		}
		return DistortPixel(p, cfg.Intrinsics, cfg.Distortion), z, true
	})
}

func (s *Scene) String() string {
	return fmt.Sprintf("%s %dx%d", s.Name, s.Raster.W, s.Raster.H)
}

func (o SceneObject) String() string {
	return fmt.Sprintf("%s depth=%.3f rect=(%d,%d)-(%d,%d)", o.Name, o.Depth, o.X0, o.Y0, o.X1, o.Y1)
}

func DepthDelta(a, b float64) float64 {
	if math.IsNaN(a) || math.IsNaN(b) {
		return math.Inf(1)
	}
	return math.Abs(a - b)
}
