// Package sim runs binocular gaze scenarios through the lens model and
// records drift, disparity, and stability metrics per compensator.
package sim

import (
	"math"

	"pupilsim/internal/gaze"
	"pupilsim/internal/optics"
)

// Frame is one binocular frame of metrics.
type Frame struct {
	T          float64
	DriftL     float64 // apparent angular drift, left eye (arcmin)
	DriftR     float64
	Disparity  float64 // |driftL - driftR| (arcmin)
	CornerMM   float64 // apparent corner displacement at the virtual image (mm)
	Jump       float64 // frame-to-frame change of mean drift (arcmin)
	OffsetJump float64 // frame-to-frame change of the applied offset (arcmin)
	Lost       bool    // either eye lost tracking
}

// Summary aggregates a scenario run.
type Summary struct {
	Scenario          string
	Compensator       string
	MaxDrift          float64 // arcmin
	RMSDrift          float64
	MaxCornerMM       float64
	MaxDisparity      float64
	RMSDisparity      float64
	MaxJump           float64 // arcmin/frame
	MaxJumpLoss       float64 // max jump on frames with tracking loss
	MaxOffsetJump     float64 // arcmin/frame, applied compensation step
	MaxOffsetJumpLoss float64 // same, on tracking-loss frames
}

// Scenario pairs a name with scripted gaze events and loss probability.
type Scenario struct {
	Name     string
	Frames   int
	Events   []gaze.Event
	LossProb float64 // overrides Config.LossProb when > 0
}

// Scenarios returns the standard evaluation set.
func Scenarios() []Scenario {
	v := func(x, y float64) gaze.Vec2 { return gaze.Vec2{X: x, Y: y} }
	return []Scenario{
		{Name: "natural", Frames: 1200},
		{Name: "saccade", Frames: 1200, Events: []gaze.Event{
			{AtFrame: 200, Target: v(3.5, 0), Duration: 6},
			{AtFrame: 400, Target: v(-3.0, 2.0), Duration: 7},
			{AtFrame: 600, Target: v(2.0, -3.0), Duration: 6},
			{AtFrame: 800, Target: v(-3.5, -1.5), Duration: 7},
		}},
		{Name: "recenter", Frames: 1200, Events: []gaze.Event{
			{AtFrame: 300, Target: v(4.0, 2.5), Duration: 7},
			{AtFrame: 700, Target: v(0, 0), Duration: 8},
		}},
		{Name: "loss", Frames: 1200, LossProb: 0.02, Events: []gaze.Event{
			{AtFrame: 400, Target: v(2.5, 1.5), Duration: 7},
			{AtFrame: 800, Target: v(0, 0), Duration: 8},
		}},
	}
}

// Run executes one scenario with one compensator factory (one instance
// per eye) and returns per-frame metrics plus the aggregate summary.
func Run(sc Scenario, lens optics.Lens, newComp func() optics.Compensator) ([]Frame, Summary) {
	cfgL := gaze.DefaultConfig()
	cfgR := gaze.DefaultConfig()
	cfgR.Seed = cfgL.Seed + 1
	if sc.LossProb > 0 {
		cfgL.LossProb = sc.LossProb
		cfgR.LossProb = sc.LossProb
	}
	genL := gaze.New(cfgL, sc.Events)
	genR := gaze.New(cfgR, sc.Events)
	compL := newComp()
	compR := newComp()

	frames := make([]Frame, 0, sc.Frames)
	sum := Summary{Scenario: sc.Name, Compensator: compL.Name()}
	var prevMean float64
	var prevOffL, prevOffR gaze.Vec2
	var sumDrift, sumDisp float64
	for i := 0; i < sc.Frames; i++ {
		sL := genL.Next()
		sR := genR.Next()
		offL := compL.Update(sL)
		offR := compR.Update(sR)
		resL := gaze.Vec2{X: sL.Pupil.X - offL.X, Y: sL.Pupil.Y - offL.Y}
		resR := gaze.Vec2{X: sR.Pupil.X - offR.X, Y: sR.Pupil.Y - offR.Y}
		dL := lens.ShiftArcMin(resL)
		dR := lens.ShiftArcMin(resR)
		mL := math.Hypot(dL.X, dL.Y)
		mR := math.Hypot(dR.X, dR.Y)
		disp := math.Hypot(dL.X-dR.X, dL.Y-dR.Y)
		cornerL := lens.ShiftMM(resL)
		cornerR := lens.ShiftMM(resR)
		corner := math.Max(math.Hypot(cornerL.X, cornerL.Y), math.Hypot(cornerR.X, cornerR.Y))
		mean := (mL + mR) / 2
		jump := 0.0
		if i > 0 {
			jump = math.Abs(mean - prevMean)
		}
		prevMean = mean
		lost := sL.Lost || sR.Lost
		offJumpL := lens.ShiftArcMin(gaze.Vec2{X: offL.X - prevOffL.X, Y: offL.Y - prevOffL.Y})
		offJumpR := lens.ShiftArcMin(gaze.Vec2{X: offR.X - prevOffR.X, Y: offR.Y - prevOffR.Y})
		offJump := math.Max(math.Hypot(offJumpL.X, offJumpL.Y), math.Hypot(offJumpR.X, offJumpR.Y))
		if i == 0 {
			offJump = 0
		}
		prevOffL, prevOffR = offL, offR
		frames = append(frames, Frame{
			T: sL.T, DriftL: mL, DriftR: mR, Disparity: disp,
			CornerMM: corner, Jump: jump, OffsetJump: offJump, Lost: lost,
		})
		m := math.Max(mL, mR)
		sum.MaxDrift = math.Max(sum.MaxDrift, m)
		sum.MaxCornerMM = math.Max(sum.MaxCornerMM, corner)
		sum.MaxDisparity = math.Max(sum.MaxDisparity, disp)
		sum.MaxJump = math.Max(sum.MaxJump, jump)
		sum.MaxOffsetJump = math.Max(sum.MaxOffsetJump, offJump)
		if lost {
			sum.MaxJumpLoss = math.Max(sum.MaxJumpLoss, jump)
			sum.MaxOffsetJumpLoss = math.Max(sum.MaxOffsetJumpLoss, offJump)
		}
		sumDrift += mean * mean
		sumDisp += disp * disp
	}
	n := float64(sc.Frames)
	sum.RMSDrift = math.Sqrt(sumDrift / n)
	sum.RMSDisparity = math.Sqrt(sumDisp / n)
	return frames, sum
}
