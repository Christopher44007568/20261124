package gazedenoise

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
)

// SceneKind selects one deterministic boundary stress case.
type SceneKind string

const (
	SceneBoundaryStep SceneKind = "boundary-step"
	SceneThinRod      SceneKind = "thin-rod"
	SceneTranslucent  SceneKind = "translucent-edge"
	SceneExposureJump SceneKind = "exposure-jump"
	SceneCombined     SceneKind = "combined"
)

// SceneKinds returns all cases in report order.
func SceneKinds() []SceneKind {
	return []SceneKind{
		SceneBoundaryStep,
		SceneThinRod,
		SceneTranslucent,
		SceneExposureJump,
		SceneCombined,
	}
}

// ParseSceneKind validates a command-line scene name.
func ParseSceneKind(s string) (SceneKind, error) {
	for _, kind := range SceneKinds() {
		if string(kind) == s {
			return kind, nil
		}
	}
	return "", fmt.Errorf("unknown scene %q (want one of %v)", s, SceneKinds())
}

// GazeEllipse describes the centre gaze mask.
type GazeEllipse struct {
	CX float64
	CY float64
	RX float64
	RY float64
}

// Contains reports whether a pixel centre lies in the gaze ellipse.
func (e GazeEllipse) Contains(x, y int) bool {
	dx := (float64(x) + 0.5 - e.CX) / e.RX
	dy := (float64(y) + 0.5 - e.CY) / e.RY
	return dx*dx+dy*dy <= 1
}

// BoundaryDistance approximates the distance in pixels to the ellipse edge.
func (e GazeEllipse) BoundaryDistance(x, y int) float64 {
	dx := (float64(x) + 0.5 - e.CX) / e.RX
	dy := (float64(y) + 0.5 - e.CY) / e.RY
	scale := math.Min(e.RX, e.RY)
	return math.Abs(math.Hypot(dx, dy)-1) * scale
}

// Scene contains clean and noisy observations plus audit masks.
type Scene struct {
	Width  int
	Height int
	Kind   SceneKind
	Seed   int64
	Noise  float64

	Gaze  GazeEllipse
	Truth *Frame
	Noisy *Frame

	// Features maps a named stress case to the pixels it affects. The mask
	// is never used by the denoiser; it only scopes validation metrics.
	Features map[string][]bool
	// BoundaryDistance is distance to the gaze ellipse in pixels.
	BoundaryDistance []float64
}

// Feature returns a named stress mask, or nil when absent.
func (s *Scene) Feature(name string) []bool {
	if s == nil {
		return nil
	}
	return s.Features[name]
}

// GenerateScene creates a deterministic truth/input pair. Noise is the per-
// channel standard deviation of zero-mean Gaussian colour noise.
func GenerateScene(kind SceneKind, width, height int, noise float64, seed int64) (*Scene, error) {
	if width < 24 || height < 24 {
		return nil, fmt.Errorf("scene must be at least 24x24, got %dx%d", width, height)
	}
	if noise < 0 || math.IsNaN(noise) || math.IsInf(noise, 0) {
		return nil, fmt.Errorf("invalid noise sigma %v", noise)
	}
	validKind := false
	for _, k := range SceneKinds() {
		if k == kind {
			validKind = true
			break
		}
	}
	if !validKind {
		return nil, fmt.Errorf("unknown scene %q", kind)
	}

	scene := &Scene{
		Width:    width,
		Height:   height,
		Kind:     kind,
		Seed:     seed,
		Noise:    noise,
		Features: make(map[string][]bool),
		Gaze: GazeEllipse{
			CX: float64(width) * 0.5,
			CY: float64(height) * 0.5,
			RX: math.Max(8, float64(width)*0.245),
			RY: math.Max(7, float64(height)*0.30),
		},
	}
	scene.BoundaryDistance = make([]float64, width*height)

	base := makeSceneSurface(scene)
	applySceneKind(scene, base)
	scene.Truth = base
	scene.Truth.RecomputeLuma()

	scene.Noisy = scene.Truth.Clone()
	addColourNoise(scene.Noisy, noise, seed)
	addHoles(scene.Noisy, scene.Gaze, seed)
	return scene, nil
}

func makeSceneSurface(s *Scene) *Frame {
	f, _ := NewFrame(s.Width, s.Height)
	w1 := math.Max(1, float64(s.Width-1))
	h1 := math.Max(1, float64(s.Height-1))
	for y := 0; y < s.Height; y++ {
		v := float64(y) / h1
		for x := 0; x < s.Width; x++ {
			u := float64(x) / w1
			i := f.Index(x, y)
			s.BoundaryDistance[i] = s.Gaze.BoundaryDistance(x, y)

			detail := 0.5 + 0.5*math.Sin(9*u+3*v)*math.Cos(5*v-2*u)
			texture := 0.018 * (0.5 + 0.5*math.Sin(41*u+17*v)*math.Sin(31*v-13*u))
			if s.Gaze.Contains(x, y) {
				f.Mask[i] = true
				f.Color[i] = Vec3{
					X: 0.46 + 0.07*u + 0.025*detail + texture,
					Y: 0.50 + 0.05*v + 0.025*detail + texture,
					Z: 0.39 + 0.04*u - 0.020*detail + texture,
				}
				f.Depth[i] = 3.20 + 0.18*(u-v) + 0.03*math.Sin(12*u)
				f.Normal[i] = Vec3{
					X: 0.24 + 0.05*math.Sin(3*v),
					Y: -0.16 + 0.04*math.Cos(4*u),
					Z: 1,
				}.Normalized()
			} else {
				f.Color[i] = Vec3{
					X: 0.27 + 0.07*u + 0.020*detail,
					Y: 0.37 + 0.06*v + 0.020*detail,
					Z: 0.44 + 0.05*(1-u) + 0.015*detail,
				}
				f.Depth[i] = 4.35 + 0.22*(u-v) + 0.05*math.Sin(7*u+3*v)
				f.Normal[i] = Vec3{
					X: -0.14 + 0.04*math.Cos(5*u),
					Y: 0.18 + 0.03*math.Sin(4*v),
					Z: 1,
				}.Normalized()
			}
		}
	}
	return f
}

func applySceneKind(s *Scene, f *Frame) {
	switch s.Kind {
	case SceneBoundaryStep:
		return
	case SceneThinRod:
		applyThinRod(s, f)
	case SceneTranslucent:
		applyTranslucentEdge(s, f)
	case SceneExposureJump:
		applyExposureJump(s, f)
	case SceneCombined:
		applyThinRod(s, f)
		applyTranslucentEdge(s, f)
		applyExposureJump(s, f)
	}
}

// applyThinRod places a high-contrast rod through both gaze-boundary
// crossings. The anti-aliased fringe tests whether a filter can preserve a
// feature narrower than its spatial support.
func applyThinRod(s *Scene, f *Frame) {
	feature := make([]bool, len(f.Color))
	x0 := 0.10 * float64(s.Width)
	y0 := 0.72 * float64(s.Height)
	x1 := 0.90 * float64(s.Width)
	y1 := 0.28 * float64(s.Height)
	rodColor := Vec3{X: 0.88, Y: 0.86, Z: 0.72}
	rodNormal := Vec3{X: -0.35, Y: 0.45, Z: 1}.Normalized()

	for y := 0; y < s.Height; y++ {
		for x := 0; x < s.Width; x++ {
			i := f.Index(x, y)
			d := pointSegmentDistance(float64(x)+0.5, float64(y)+0.5, x0, y0, x1, y1)
			alpha := Clamp01((2.10-d)/1.05) * 0.95
			if alpha <= 0 {
				continue
			}
			f.Color[i] = mixVec(f.Color[i], rodColor, alpha)
			f.Depth[i] = mix(f.Depth[i], 1.82+0.02*math.Sin(5*float64(x)/float64(s.Width)), alpha)
			f.Normal[i] = mixVec(f.Normal[i], rodNormal, alpha).Normalized()
			feature[i] = alpha > 0.05
		}
	}
	s.Features["thin_rod"] = feature
}

// applyTranslucentEdge overlays a soft glass pane whose edge cuts the gaze
// mask. Its blended depth and normal are intentionally ambiguous.
func applyTranslucentEdge(s *Scene, f *Frame) {
	feature := make([]bool, len(f.Color))
	tint := Vec3{X: 0.74, Y: 0.84, Z: 0.91}
	glassNormal := Vec3{X: 0.08, Y: -0.05, Z: 1}.Normalized()
	h1 := math.Max(1, float64(s.Height-1))

	for y := 0; y < s.Height; y++ {
		v := float64(y) / h1
		centre := 0.585 + 0.035*math.Sin(6*v)
		halfWidth := 0.055
		for x := 0; x < s.Width; x++ {
			i := f.Index(x, y)
			u := float64(x) / math.Max(1, float64(s.Width-1))
			d := math.Abs(u-centre) / halfWidth
			alpha := 0.42 * smoothstep(1.0, 0.72, d)
			if alpha <= 0.01 {
				continue
			}
			f.Color[i] = mixVec(f.Color[i], tint, alpha)
			f.Depth[i] = mix(f.Depth[i], 2.55, alpha*0.55)
			f.Normal[i] = mixVec(f.Normal[i], glassNormal, alpha*0.35).Normalized()
			f.Luma[i] = f.Color[i].Luma()
			feature[i] = true
		}
	}
	s.Features["translucent_edge"] = feature
}

// applyExposureJump changes exposure on one side of a hard cut. Geometry is
// unchanged there, so only the luminance guide identifies the discontinuity.
func applyExposureJump(s *Scene, f *Frame) {
	feature := make([]bool, len(f.Color))
	jumpX := s.Gaze.CX + s.Gaze.RX*0.55
	for y := 0; y < s.Height; y++ {
		for x := 0; x < s.Width; x++ {
			i := f.Index(x, y)
			d := (float64(x) + 0.5) - jumpX
			alpha := smoothstep(-0.75, 0.75, d)
			if alpha <= 0 {
				continue
			}
			f.Color[i] = f.Color[i].Mul(1 + 0.72*alpha)
			f.Luma[i] = f.Color[i].Luma()
			if math.Abs(d) <= 2.0 {
				feature[i] = true
			}
		}
	}
	s.Features["exposure_jump"] = feature
}

func pointSegmentDistance(px, py, x0, y0, x1, y1 float64) float64 {
	dx, dy := x1-x0, y1-y0
	denom := dx*dx + dy*dy
	t := 0.0
	if denom > 0 {
		t = ((px-x0)*dx + (py-y0)*dy) / denom
	}
	t = Clamp01(t)
	return math.Hypot(px-(x0+t*dx), py-(y0+t*dy))
}

func smoothstep(edge0, edge1, x float64) float64 {
	if edge0 == edge1 {
		if x < edge0 {
			return 0
		}
		return 1
	}
	t := Clamp01((x - edge0) / (edge1 - edge0))
	return t * t * (3 - 2*t)
}

func mix(a, b, t float64) float64 {
	return a + (b-a)*t
}

func mixVec(a, b Vec3, t float64) Vec3 {
	return Vec3{
		X: mix(a.X, b.X, t),
		Y: mix(a.Y, b.Y, t),
		Z: mix(a.Z, b.Z, t),
	}
}

// FeatureNames returns stress-mask names in stable order.
func (s *Scene) FeatureNames() []string {
	names := make([]string, 0, len(s.Features))
	for name := range s.Features {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func addColourNoise(f *Frame, sigma float64, seed int64) {
	if sigma == 0 {
		f.RecomputeLuma()
		return
	}
	rng := rand.New(rand.NewSource(seed ^ 0x6d2b79f5))
	gauss := func() float64 {
		u1 := math.Max(rng.Float64(), 1e-12)
		u2 := rng.Float64()
		return math.Sqrt(-2*math.Log(u1)) * math.Cos(2*math.Pi*u2)
	}
	for i := range f.Color {
		f.Color[i] = Vec3{
			X: Clamp(f.Color[i].X+sigma*gauss(), 0, 2),
			Y: Clamp(f.Color[i].Y+sigma*gauss(), 0, 2),
			Z: Clamp(f.Color[i].Z+sigma*gauss(), 0, 2),
		}
	}
	f.RecomputeLuma()
}

// addHoles marks isolated missing samples and a few two-by-two clumps on
// both sides of the gaze boundary. They exercise provenance-aware filling.
func addHoles(f *Frame, gaze GazeEllipse, seed int64) {
	rng := rand.New(rand.NewSource(seed ^ 0x51ed270b))
	for i := range f.Valid {
		if rng.Intn(421) == 0 {
			f.Valid[i] = false
		}
	}

	angles := []float64{0, math.Pi / 3, 2 * math.Pi / 3, math.Pi, 4 * math.Pi / 3, 5 * math.Pi / 3}
	for k, angle := range angles {
		scale := 0.965
		if k%2 == 1 {
			scale = 1.035
		}
		x := int(math.Round(gaze.CX + math.Cos(angle)*gaze.RX*scale))
		y := int(math.Round(gaze.CY + math.Sin(angle)*gaze.RY*scale))
		for dy := 0; dy < 2; dy++ {
			for dx := 0; dx < 2; dx++ {
				if f.InBounds(x+dx, y+dy) {
					f.Valid[f.Index(x+dx, y+dy)] = false
				}
			}
		}
	}
}
