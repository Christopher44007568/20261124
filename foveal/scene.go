package foveal

// This file is an in-memory, dependency-free test scene. It renders a static
// background, a moving foreground square (with real disocclusion behind it),
// a roaming specular highlight, global exposure changes and additive Gaussian
// noise, and provides the exact motion vectors / depth / invalid hints the
// denoiser consumes.

// MovingObject describes the foreground square in scene coordinates.
type MovingObject struct {
	X, Y         float32 // top-left corner, pixels
	Size         int
	SpeedX       float32 // px per frame
	Color        [3]float32
	Depth        float32
	PrevX, PrevY float32
}

// Highlight is a bright spot that can appear, move and disappear.
type Highlight struct {
	X, Y    float32
	Radius  float32
	Color   [3]float32
	Enabled bool
}

// Scene is a self-describing animated frame source.
type Scene struct {
	Width, Height int

	Background      func(x, y int) [3]float32
	BackgroundDepth float32

	Object   MovingObject
	Spot     Highlight
	Gaze     Vec2
	Exposure float32 // multiplier applied to the whole frame

	NoiseSigma    float32
	rngState      uint64
	frameIndex    int
	prevExposure  float32
	spareGaussian float32
}

// NewScene builds a deterministic scene with a smoothly varying background.
func NewScene(width, height int) *Scene {
	s := &Scene{
		Width:           width,
		Height:          height,
		BackgroundDepth: 10,
		Object: MovingObject{
			Size:  max(width, height) / 5,
			Depth: 3,
			Color: [3]float32{0.8, 0.3, 0.2},
		},
		Spot: Highlight{
			Radius: float32(max(width, height)) * 0.05,
			Color:  [3]float32{4, 4, 4},
		},
		Exposure:   1,
		NoiseSigma: 0.1,
		rngState:   0x9e3779b97f4a7c15,
	}
	s.Background = func(x, y int) [3]float32 {
		fx := float32(x) / float32(width)
		fy := float32(y) / float32(height)
		return [3]float32{
			0.25 + 0.25*fx,
			0.2 + 0.3*fy,
			0.35 + 0.2*(1-fx),
		}
	}
	return s
}

// Step advances object and highlight state by one frame.
func (s *Scene) Step() {
	s.frameIndex++
	s.Object.PrevX = s.Object.X
	s.Object.PrevY = s.Object.Y
	s.Object.X += s.Object.SpeedX

	// Bounce inside the frame horizontally.
	right := s.Object.X + float32(s.Object.Size)
	if right > float32(s.Width) || s.Object.X < 0 {
		s.Object.SpeedX = -s.Object.SpeedX
		s.Object.X = max(0, min(float32(s.Width-s.Object.Size), s.Object.X))
	}
}

func (s *Scene) insideObjectAt(x, y int, ox, oy float32) bool {
	return float32(x) >= ox && float32(x) < ox+float32(s.Object.Size) &&
		float32(y) >= oy && float32(y) < oy+float32(s.Object.Size)
}

// Render produces one noisy Frame plus a clean reference of the same scene.
func (s *Scene) Render() (frame *Frame, clean []float32, invalid []bool) {
	w, h := s.Width, s.Height
	n := w * h
	frame = &Frame{
		Width:         w,
		Height:        h,
		RGB:           make([]float32, n*3),
		MV:            make([]Vec2, n),
		Depth:         make([]float32, n),
		ExposureScale: s.exposureScale(),
		Gaze:          s.Gaze,
	}
	clean = make([]float32, n*3)
	invalid = make([]bool, n)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			p := y*w + x
			col, depth, mv, hidden := s.sample(x, y)
			frame.Depth[p] = depth
			frame.MV[p] = mv
			moving := s.Object.X != s.Object.PrevX || s.Object.Y != s.Object.PrevY
			if hidden && moving {
				invalid[p] = true
			}
			for c := 0; c < 3; c++ {
				noisy := (col[c] + s.gaussian()) * s.Exposure
				frame.RGB[p*3+c] = noisy
				clean[p*3+c] = col[c] * s.Exposure
			}
		}
	}
	frame.Invalid = invalid
	return frame, clean, invalid
}

// sample returns color, depth, motion vector (previous position offset) and
// whether the pixel is a newly revealed disocclusion.
func (s *Scene) sample(x, y int) ([3]float32, float32, Vec2, bool) {
	o := s.Object
	if s.insideObjectAt(x, y, o.X, o.Y) {
		mv := Vec2{o.X - o.PrevX, o.Y - o.PrevY}
		return o.Color, o.Depth, mv, false
	}

	// Revealed background: background now, previously covered by the object.
	moved := o.X != o.PrevX || o.Y != o.PrevY
	hidden := moved && s.insideObjectAt(x, y, o.PrevX, o.PrevY)
	col := s.Background(x, y)

	if s.Spot.Enabled {
		dx := float32(x) - s.Spot.X
		dy := float32(y) - s.Spot.Y
		if d2 := dx*dx + dy*dy; d2 <= s.Spot.Radius*s.Spot.Radius {
			falloff := 1 - sqrt32(d2)/s.Spot.Radius
			for c := 0; c < 3; c++ {
				col[c] += s.Spot.Color[c] * falloff
			}
		}
	}
	return col, s.BackgroundDepth, Vec2{}, hidden
}

// exposureScale returns current/previous exposure; the scene tracks exposure
// changes through SetExposure between renders.
func (s *Scene) exposureScale() float32 {
	if s.prevExposure <= 0 {
		return 1
	}
	return s.Exposure / s.prevExposure
}

// SetExposure updates exposure while remembering the previous value.
func (s *Scene) SetExposure(e float32) {
	s.prevExposure = s.Exposure
	s.Exposure = e
}

// splitmix64-based Gaussian generator (Box-Muller), deterministic and
// dependency-free.
func (s *Scene) gaussian() float32 {
	if s.spareGaussian != 0 {
		v := s.spareGaussian
		s.spareGaussian = 0
		return v * s.NoiseSigma
	}
	u1 := max(s.nextUnit(), 1e-12)
	u2 := s.nextUnit()
	r := sqrt32(-2 * log32(u1))
	theta := 2 * pi32 * u2
	s.spareGaussian = r * sin32(theta)
	return r * cos32(theta) * s.NoiseSigma
}
