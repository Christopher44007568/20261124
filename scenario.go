package reprojection

import (
	"fmt"
	"math"
	"time"
)

// Mode selects one of the three composition policies under test.
type Mode string

const (
	// Layered keeps world and system UI in separate layers. Both are resolved
	// for the same scanout instant; only the world consults its depth buffer.
	Layered Mode = "layered"
	// MergedDepth merges UI into the world depth buffer using the UI depth
	// scale. It is intentionally unsafe for system UI.
	MergedDepth Mode = "merged-depth"
	// InheritedWorldCamera treats fixed UI as world geometry and reuses the
	// world's render-time camera. It is intentionally unsafe as well.
	InheritedWorldCamera Mode = "inherited-world-camera"
)

func ParseMode(s string) (Mode, error) {
	switch Mode(s) {
	case Layered, MergedDepth, InheritedWorldCamera:
		return Mode(s), nil
	default:
		return "", fmt.Errorf("unknown composite mode %q", s)
	}
}

func AllModes() []Mode {
	return []Mode{Layered, MergedDepth, InheritedWorldCamera}
}

// Scenario is a deterministic head/display/hand simulation. HeadMotion and
// HandMotion may be replaced by tests; nil uses rapid-turn defaults.
type Scenario struct {
	Camera         Camera
	Panels         []WorldPanel
	UI             UILayer
	WorldLatency   time.Duration
	UIInputLatency time.Duration
	HeadMotion     func(time.Duration) Pose
	HandMotion     func(time.Duration) Vec3
}

func DefaultScenario() Scenario {
	return Scenario{
		Camera:         NewCamera(480, 270, 60*math.Pi/180),
		Panels:         DefaultWorld(),
		UI:             DefaultUI(),
		WorldLatency:   11 * time.Millisecond,
		UIInputLatency: 4 * time.Millisecond,
	}
}

func defaultHead(t time.Duration) Pose {
	s := t.Seconds()
	return Pose{
		Yaw:   0.31*math.Sin(2*math.Pi*2.3*s) + 0.035*math.Sin(2*math.Pi*7.7*s+0.3),
		Pitch: 0.046*math.Sin(2*math.Pi*1.6*s+0.7) + 0.012*math.Sin(2*math.Pi*5.3*s),
	}
}

func defaultHand(t time.Duration) Vec3 {
	s := t.Seconds()
	return Vec3{
		X: 0.16 + 0.15*math.Sin(2*math.Pi*1.9*s+0.4),
		Y: -0.04 + 0.08*math.Cos(2*math.Pi*1.45*s),
		Z: 1.05,
	}
}

func (s Scenario) headAt(t time.Duration) Pose {
	if s.HeadMotion != nil {
		return s.HeadMotion(t)
	}
	return defaultHead(t)
}

func (s Scenario) HeadAt(t time.Duration) Pose {
	return s.headAt(t)
}

func (s Scenario) handAt(t time.Duration) Vec3 {
	if s.HandMotion != nil {
		return s.HandMotion(t)
	}
	return defaultHand(t)
}

func (s Scenario) HandAt(t time.Duration) Vec3 {
	return s.handAt(t)
}

func (s Scenario) prepare() (Scenario, error) {
	if s.Camera.Width <= 0 || s.Camera.Height <= 0 || s.Camera.VFOV <= 0 {
		return s, fmt.Errorf("invalid camera %dx%d vfov=%g", s.Camera.Width, s.Camera.Height, s.Camera.VFOV)
	}
	if len(s.Panels) == 0 {
		s.Panels = DefaultWorld()
	}
	if s.UI.Reticle.Radius <= 0 && s.UI.Ruler.Width <= 0 && s.UI.Cursor.Radius <= 0 {
		s.UI = DefaultUI()
	}
	if s.WorldLatency < 0 {
		s.WorldLatency = 0
	}
	if s.UIInputLatency < 0 {
		s.UIInputLatency = 0
	}
	return s, nil
}

func earlier(t time.Duration, d time.Duration) time.Duration {
	t -= d
	if t < 0 {
		return 0
	}
	return t
}

func durationMs(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}

// FrameTiming exposes the clocks that a compositor must keep straight.
type FrameTiming struct {
	Scanout     time.Duration
	WorldRender time.Duration
	UIInput     time.Duration
}

func (s Scenario) Timing(scanout time.Duration) FrameTiming {
	return FrameTiming{
		Scanout:     scanout,
		WorldRender: earlier(scanout, s.WorldLatency),
		UIInput:     earlier(scanout, s.UIInputLatency),
	}
}

// FrameResult is one complete scanout raster and its measured properties.
type FrameResult struct {
	Mode        Mode
	Timing      FrameTiming
	UIScanout   time.Duration
	UIInputTime time.Duration
	Buffer      *FrameBuffer
	Reference   *FrameBuffer
	World       *WorldImage
	GroundTruth *WorldImage
	UI          *UIImage
	UIMask      []bool
	UIRejected  []bool
	Anchors     []UIAnchor
	IdealAnchor []UIAnchor
	Metrics     FrameMetrics
}

type frameBase struct {
	scenario     Scenario
	timing       FrameTiming
	scanoutPose  Pose
	renderPose   Pose
	world        *WorldImage
	truth        *WorldImage
	idealState   UIState
	idealAnchors []UIAnchor
	idealUI      *UIImage
	reference    *FrameBuffer
	worldMetric  FrameMetrics
}

func (s Scenario) cursorAt(t time.Duration, pose Pose) (Vec2, bool) {
	screen, _, ok := s.Camera.Project(s.handAt(t), pose)
	if !ok {
		return Vec2{float64(s.Camera.Width) / 2, float64(s.Camera.Height) / 2}, false
	}
	if !s.Camera.inside(screen) {
		return screen, false
	}
	return screen, true
}

func (s Scenario) renderBase(scanout time.Duration) (frameBase, error) {
	s, err := s.prepare()
	if err != nil {
		return frameBase{}, err
	}
	if scanout < 0 {
		return frameBase{}, fmt.Errorf("negative scanout time %s", scanout)
	}

	timing := s.Timing(scanout)
	scanoutPose := s.headAt(scanout)
	renderPose := s.headAt(timing.WorldRender)

	rendered := RenderWorld(s.Camera, s.Panels, renderPose)
	world := ReprojectWorld(rendered, s.Camera, scanoutPose)
	truth := RenderWorld(s.Camera, s.Panels, scanoutPose)

	cursor, visible := s.cursorAt(timing.UIInput, scanoutPose)
	state := UIState{Cursor: cursor, Visible: visible}
	idealUI := s.UI.Render(s.Camera, state)
	reference, _, _ := CompositeOverlay(world, idealUI, false)

	b := frameBase{
		scenario:     s,
		timing:       timing,
		scanoutPose:  scanoutPose,
		renderPose:   renderPose,
		world:        world,
		truth:        truth,
		idealState:   state,
		idealAnchors: s.UI.Anchors(s.Camera, state),
		idealUI:      idealUI,
		reference:    reference,
	}
	b.worldMetric = worldMetrics(world, truth)
	return b, nil
}

func transformAnchor(a UIAnchor, cam Camera, sourcePose, targetPose Pose) (UIAnchor, bool) {
	world := cam.UnprojectScreen(a.Screen, a.Depth, sourcePose)
	screen, _, ok := cam.Project(world, targetPose)
	if !ok || !cam.inside(screen) {
		return UIAnchor{}, false
	}
	a.Screen = screen
	return a, true
}

func (b frameBase) render(mode Mode) FrameResult {
	s := b.scenario
	ui := b.idealUI
	anchors := append([]UIAnchor(nil), b.idealAnchors...)
	uiScanout := b.timing.Scanout
	uiInput := b.timing.UIInput
	depthTest := false

	switch mode {
	case Layered:
		// Correct: UI remains an independent screen-space layer.
	case MergedDepth:
		depthTest = true
	case InheritedWorldCamera:
		sourcePose := b.renderPose
		sourceTime := b.timing.WorldRender
		cursor, visible := s.cursorAt(sourceTime, sourcePose)
		sourceState := UIState{Cursor: cursor, Visible: visible}
		anchors = anchors[:0]
		for _, a := range s.UI.Anchors(s.Camera, sourceState) {
			if transformed, ok := transformAnchor(a, s.Camera, sourcePose, b.scanoutPose); ok {
				anchors = append(anchors, transformed)
			}
		}
		ui = TransformUI(s.UI.Render(s.Camera, sourceState), s.Camera, sourcePose, b.scanoutPose)
		uiScanout = sourceTime
		uiInput = sourceTime
	}

	buffer, mask, rejected := CompositeOverlay(b.world, ui, depthTest)
	result := FrameResult{
		Mode:        mode,
		Timing:      b.timing,
		UIScanout:   uiScanout,
		UIInputTime: uiInput,
		Buffer:      buffer,
		Reference:   b.reference,
		World:       b.world,
		GroundTruth: b.truth,
		UI:          ui,
		UIMask:      mask,
		UIRejected:  rejected,
		Anchors:     anchors,
		IdealAnchor: b.idealAnchors,
	}
	result.Metrics = b.measure(result)
	return result
}

func findAnchor(anchors []UIAnchor, name string) (UIAnchor, bool) {
	for _, a := range anchors {
		if a.Name == name {
			return a, true
		}
	}
	return UIAnchor{}, false
}

func (b frameBase) measure(r FrameResult) FrameMetrics {
	s := b.scenario
	required := countFilled(b.idealUI)
	visible := countTrue(r.UIMask)
	rejected := countTrue(r.UIRejected)

	presenceMissing := 0
	for i := range b.idealUI.Pixels {
		u := &b.idealUI.Pixels[i]
		if u.Filled && u.Color.A > 0 && !r.UIMask[i] {
			presenceMissing++
		}
	}

	m := b.worldMetric
	m.UIRequiredPixels = float64(required)
	m.UIVisiblePixels = float64(visible)
	m.DepthRejected = float64(rejected)
	if required > 0 {
		m.DepthRejectedRate = float64(rejected) / float64(required)
		m.UIPresenceError = float64(presenceMissing) / float64(required)
	}
	if len(r.Buffer.Pixels) > 0 {
		m.ReferenceMismatch = float64(countFrameDiff(r.Buffer, b.reference)) / float64(len(r.Buffer.Pixels))
	}

	for _, name := range []string{"reticle", "ruler"} {
		want, okWant := findAnchor(b.idealAnchors, name)
		got, okGot := findAnchor(r.Anchors, name)
		if okWant && okGot {
			if d := want.Screen.Distance(got.Screen); d > m.FixedUIDriftPx {
				m.FixedUIDriftPx = d
			}
		}
	}

	expectedCursor, okExpected := s.cursorAt(b.timing.Scanout, b.scanoutPose)
	cursor, okCursor := findAnchor(r.Anchors, "cursor")
	if okExpected && okCursor {
		m.CursorErrorPx = expectedCursor.Distance(cursor.Screen)
		m.FollowLatencyMs = s.estimateLatency(cursor.Screen, b.timing.Scanout)
	}
	skew := r.UIScanout - b.timing.Scanout
	if skew < 0 {
		skew = -skew
	}
	m.ScanoutSkewMs = durationMs(skew)
	return m
}

func (s Scenario) estimateLatency(actual Vec2, scanout time.Duration) float64 {
	bestAge := 0.0
	bestDistance := math.Inf(1)
	for ageMs := 0.0; ageMs <= 80; ageMs += 0.25 {
		age := time.Duration(ageMs * float64(time.Millisecond))
		sampleTime := earlier(scanout, age)
		pos, ok := s.cursorAt(sampleTime, s.headAt(scanout))
		if !ok {
			continue
		}
		if d := pos.Distance(actual); d < bestDistance {
			bestDistance = d
			bestAge = ageMs
		}
	}
	return bestAge
}

func worldMetrics(world, truth *WorldImage) FrameMetrics {
	m := FrameMetrics{
		WorldCoverage:     world.Coverage(),
		WorldHoleRate:     1 - world.Coverage(),
		WorldNearCoverage: truth.NearCoverage(),
	}
	total := len(world.Pixels)
	if total == 0 {
		return m
	}
	agree := 0
	for i := range world.Pixels {
		gotNear := world.Pixels[i].Panel >= 0 && world.Pixels[i].Near
		wantNear := truth.Pixels[i].Panel >= 0 && truth.Pixels[i].Near
		if gotNear == wantNear {
			agree++
		}
	}
	m.NearMaskAgreement = float64(agree) / float64(total)
	return m
}

// Frame renders one scanout instant using mode.
func (s Scenario) Frame(scanout time.Duration, mode Mode) (FrameResult, error) {
	if _, err := ParseMode(string(mode)); err != nil {
		return FrameResult{}, err
	}
	b, err := s.renderBase(scanout)
	if err != nil {
		return FrameResult{}, err
	}
	return b.render(mode), nil
}

// CompareAt renders all three policies for exactly the same scanout instant.
func (s Scenario) CompareAt(scanout time.Duration) ([]FrameResult, error) {
	b, err := s.renderBase(scanout)
	if err != nil {
		return nil, err
	}
	out := make([]FrameResult, 0, 3)
	for _, mode := range AllModes() {
		out = append(out, b.render(mode))
	}
	return out, nil
}

type Summary struct {
	Mode   Mode
	Frames int
	Start  time.Duration
	Step   time.Duration
	Mean   FrameMetrics
	Max    FrameMetrics
}

func seriesTimes(start, step time.Duration, frames int) ([]time.Duration, error) {
	if frames <= 0 {
		return nil, fmt.Errorf("frames must be positive")
	}
	if step <= 0 {
		return nil, fmt.Errorf("step must be positive")
	}
	if start < 0 {
		return nil, fmt.Errorf("start must not be negative")
	}
	out := make([]time.Duration, frames)
	for i := range out {
		out[i] = start + time.Duration(i)*step
	}
	return out, nil
}

func (s Scenario) Run(mode Mode, start, step time.Duration, frames int) (Summary, error) {
	times, err := seriesTimes(start, step, frames)
	if err != nil {
		return Summary{}, err
	}
	if _, err := ParseMode(string(mode)); err != nil {
		return Summary{}, err
	}
	sum := Summary{Mode: mode, Frames: len(times), Start: start, Step: step}
	var total FrameMetrics
	for _, scanout := range times {
		r, err := s.Frame(scanout, mode)
		if err != nil {
			return Summary{}, err
		}
		total = addMetrics(total, r.Metrics)
		sum.Max = maxMetrics(sum.Max, r.Metrics)
	}
	sum.Mean = divMetrics(total, float64(len(times)))
	return sum, nil
}

// Compare runs the same scanout series through all three policies.
func (s Scenario) Compare(start, step time.Duration, frames int) ([]Summary, error) {
	times, err := seriesTimes(start, step, frames)
	if err != nil {
		return nil, err
	}
	modes := AllModes()
	sums := make([]Summary, len(modes))
	totals := make([]FrameMetrics, len(modes))
	for i, mode := range modes {
		sums[i] = Summary{Mode: mode, Frames: len(times), Start: start, Step: step}
	}
	for _, scanout := range times {
		results, err := s.CompareAt(scanout)
		if err != nil {
			return nil, err
		}
		for i := range results {
			totals[i] = addMetrics(totals[i], results[i].Metrics)
			sums[i].Max = maxMetrics(sums[i].Max, results[i].Metrics)
		}
	}
	for i := range sums {
		sums[i].Mean = divMetrics(totals[i], float64(len(times)))
	}
	return sums, nil
}
