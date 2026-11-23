package ae

import "math"

const (
	DefaultWidth  = 96
	DefaultHeight = 72

	scenarioFrames = 40
	stepFrame      = 8
	flashOffFrame  = 10
	baseLuma       = 0.40
)

// Scenario is a deterministic luminance sequence with one primary scene
// transition. Flash has a second transition when the flash turns off, so
// LastTransition identifies the point from which its recovery is measured.
type Scenario struct {
	Name           string
	Description    string
	Frames         []Frame
	LastTransition int
	BaseLuma       float64
}

func Scenarios() []Scenario {
	return []Scenario{
		makeScenario("black-field", "mid-grey to a 0.02 black field",
			func(f *Frame, index int) {
				if index >= stepFrame {
					fill(f, 0.02)
				}
			}, stepFrame),
		makeScenario("white-field", "mid-grey to a 0.95 white field",
			func(f *Frame, index int) {
				if index >= stepFrame {
					fill(f, 0.95)
				}
			}, stepFrame),
		makeScenario("flash", "two-frame full-field flash followed by recovery",
			func(f *Frame, index int) {
				if index >= stepFrame && index < flashOffFrame {
					fill(f, 1)
				}
				if index == flashOffFrame {
					f.SceneCut = true
				}
			}, flashOffFrame),
		makeScenario("local-highlight", "a persistent bright patch on a mid-grey field",
			func(f *Frame, index int) {
				if index >= stepFrame {
					drawHighlight(f)
				}
			}, stepFrame),
	}
}

func makeScenario(name, description string, mutate func(*Frame, int), lastTransition int) Scenario {
	frames := make([]Frame, scenarioFrames)
	for i := range frames {
		frame := NewUniformFrame(DefaultWidth, DefaultHeight, baseLuma, i == stepFrame)
		mutate(&frame, i)
		frame.Label = name
		frames[i] = frame
	}
	return Scenario{
		Name:           name,
		Description:    description,
		Frames:         frames,
		LastTransition: lastTransition,
		BaseLuma:       baseLuma,
	}
}

func fill(frame *Frame, value float64) {
	for i := range frame.Luma {
		frame.Luma[i] = value
	}
}

func drawHighlight(frame *Frame) {
	// The patch covers about 8.6% of the frame but only 1/8 of the zone
	// grid, so a zone median rejects it while a whole-frame mean does not.
	x0, x1 := frame.Width*2/3, frame.Width*8/9
	y0, y1 := frame.Height/6, frame.Height*5/9
	for y := y0; y < y1; y++ {
		row := y * frame.Width
		for x := x0; x < x1; x++ {
			frame.Luma[row+x] = 1
		}
	}
}

func InitialEV(scenario Scenario, cfg ControlConfig) float64 {
	return math.Log2(cfg.Target / scenario.BaseLuma)
}
