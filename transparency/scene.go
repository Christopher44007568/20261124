package transparency

import "math"

// Scene builds a deterministic CPU-only demo. It contains three transparent
// layers at different depths: drifting fog, a moving glass pane, and a thin
// filament in front of both.
type Scene struct {
	Width  int
	Height int
}

func NewDemoScene(width, height int) *Scene {
	return &Scene{Width: width, Height: height}
}

// BackgroundAt returns the opaque background sampled at the pixel center.
// Keeping it smooth prevents background noise from hiding temporal artifacts.
func (s *Scene) BackgroundAt(x, y int) RGB {
	u := float64(x) / float64(maxInt(1, s.Width-1))
	v := float64(y) / float64(maxInt(1, s.Height-1))
	vignette := math.Pow(math.Max(0, 1-0.85*math.Hypot(u-0.5, v-0.5)), 2)
	return RGB{
		R: 0.035 + 0.10*u + 0.045*v,
		G: 0.055 + 0.16*v + 0.035*u,
		B: 0.08 + 0.20*(1-u) + 0.08*v,
	}.Mul(0.65 + 0.35*vignette)
}

// Jitter returns the deterministic subpixel offset for a frame.
func Jitter(frame int) Vec2 {
	const g = 2.324717957244746
	return Vec2{
		X: fract(0.5+g*float64(frame)) - 0.5,
		Y: fract(0.5+g*g*float64(frame)) - 0.5,
	}
}

func fract(v float64) float64 { return v - math.Floor(v) }

// HistoryWeightFor gives every transparent layer its own temporal weight.
// Depth lowers confidence because a far layer has a different geometric
// interpretation than a near layer. Screen-space speed lowers it further.
func HistoryWeightFor(depth float64, motion Vec2) float64 {
	depthTerm := 0.18 * math.Log1p(math.Max(depth, 0)) / math.Log1p(32)
	motionTerm := 0.20 * clamp(motion.Length()/2, 0, 1)
	return clamp(0.92-depthTerm-motionTerm, 0.25, 0.90)
}

// LayersAt constructs one frame of transparent layers. Motion is measured from
// the previous frame and is used only for that layer's reprojection.
func (s *Scene) LayersAt(frame int) []*Layer {
	t := float64(frame)

	fogShift := Vec2{0.65 * t, 0.21 * t}
	fog := &Layer{
		ID:             "fog",
		Kind:           KindFog,
		Depth:          26,
		DepthTolerance: 0.25,
		Opacity:        0.13,
		Color:          RGB{0.58, 0.70, 0.82},
		Motion:         Vec2{0.65, 0.21},
		Shape:          FullScreenShape{},
		DepthAt:        func(Vec2) float64 { return 26 },
		Shade: func(p Vec2) float64 {
			q := p.Sub(fogShift)
			n := 0.5 + 0.25*math.Sin(q.X*0.21+q.Y*0.13) +
				0.25*math.Sin(q.X*0.09-q.Y*0.27+1.7)
			return 0.55 + 0.90*clamp(n, 0, 1)
		},
	}
	fog.HistoryWeight = HistoryWeightFor(fog.Depth, fog.Motion)

	glassCenter, prevGlassCenter := s.glassCenter(frame), s.glassCenter(frame-1)
	glassHalf := Vec2{0.17 * float64(s.Width), 0.14 * float64(s.Height)}
	glassAngle := 0.20 + 0.08*math.Sin(0.09*t)
	glassShape := RotatedRect{Center: glassCenter, Half: glassHalf, Angle: glassAngle}
	glass := &Layer{
		ID:             "glass",
		Kind:           KindGlass,
		Depth:          4.5,
		DepthTolerance: 0.40,
		Opacity:        0.38,
		Color:          RGB{0.32, 0.84, 0.96},
		Motion:         glassCenter.Sub(prevGlassCenter),
		Shape:          glassShape,
		DepthAt: func(p Vec2) float64 {
			q := glassShape.LocalPoint(p)
			return 4.5 + 0.35*clamp(q.Y/glassHalf.Y, -1, 1)
		},
		Shade: func(p Vec2) float64 {
			q := glassShape.LocalPoint(p)
			return 0.86 + 0.14*math.Sin(q.X*0.23+q.Y*0.11)
		},
	}
	glass.HistoryWeight = HistoryWeightFor(glass.Depth, glass.Motion)

	filamentA, filamentB, prevFilamentCenter := s.filament(frame)
	filamentCenter := filamentA.Add(filamentB).Mul(0.5)
	filamentShape := Segment{A: filamentA, B: filamentB, Radius: 0.55}
	filament := &Layer{
		ID:             "filament",
		Kind:           KindFilament,
		Depth:          1.2,
		DepthTolerance: 0.35,
		Opacity:        0.94,
		Color:          RGB{1.0, 0.84, 0.34},
		Motion:         filamentCenter.Sub(prevFilamentCenter),
		Shape:          filamentShape,
		DepthAt:        func(Vec2) float64 { return 1.2 },
		Shade:          func(Vec2) float64 { return 1 },
	}
	filament.HistoryWeight = HistoryWeightFor(filament.Depth, filament.Motion)

	return []*Layer{fog, glass, filament}
}

func (s *Scene) glassCenter(frame int) Vec2 {
	t := float64(frame)
	return Vec2{
		X: 0.52*float64(s.Width) + 0.15*float64(s.Width)*math.Sin(0.13*t),
		Y: 0.50*float64(s.Height) + 0.10*float64(s.Height)*math.Cos(0.11*t),
	}
}

func (s *Scene) filament(frame int) (Vec2, Vec2, Vec2) {
	t := float64(frame)
	center := Vec2{
		X: 0.48*float64(s.Width) + 0.17*float64(s.Width)*math.Sin(0.17*t+1.2),
		Y: 0.52*float64(s.Height) + 0.16*float64(s.Height)*math.Sin(0.21*t),
	}
	prev := Vec2{
		X: 0.48*float64(s.Width) + 0.17*float64(s.Width)*math.Sin(0.17*float64(frame-1)+1.2),
		Y: 0.52*float64(s.Height) + 0.16*float64(s.Height)*math.Sin(0.21*float64(frame-1)),
	}
	angle := 0.82 + 0.12*math.Sin(0.13*t)
	half := 0.27 * math.Min(float64(s.Width), float64(s.Height))
	d := Vec2{math.Cos(angle) * half, math.Sin(angle) * half}
	return center.Sub(d), center.Add(d), prev
}

// BlockMotion is the single motion a naive whole-block resolve can observe.
// It is a coverage-agnostic average, so it cannot be correct for all depths.
func (s *Scene) BlockMotion(frame int) Vec2 {
	layers := s.LayersAt(frame)
	var sum Vec2
	for _, layer := range layers {
		sum = sum.Add(layer.Motion)
	}
	return sum.Mul(1 / float64(len(layers)))
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
