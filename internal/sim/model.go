package sim

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Eye int

const (
	Left Eye = iota
	Right
)

var Eyes = [...]Eye{Left, Right}

func (e Eye) String() string {
	if e == Left {
		return "left"
	}
	return "right"
}

func (e Eye) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.String())
}

type Region int

const (
	Center Region = iota
	Periphery
)

var Regions = [...]Region{Center, Periphery}

func (r Region) String() string {
	if r == Center {
		return "center"
	}
	return "periphery"
}

func (r Region) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.String())
}

type Tier int

const (
	TierMin Tier = iota
	TierMid
	TierFull
)

func (t Tier) String() string {
	switch t {
	case TierMin:
		return "min"
	case TierMid:
		return "mid"
	case TierFull:
		return "full"
	default:
		return "unknown"
	}
}

func (t Tier) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.String())
}

type TierSpec struct {
	Samples   int     `json:"samples"`
	EyeBuffer int     `json:"eye_buffer"`
	Redraw    int     `json:"redraw"`
	Error     float64 `json:"error"`
}

type Profile struct {
	Center    [3]TierSpec `json:"center"`
	Periphery [3]TierSpec `json:"periphery"`
}

func DefaultProfile() Profile {
	return Profile{
		Center: [3]TierSpec{
			{Samples: 4, EyeBuffer: 2, Redraw: 1, Error: 5.0},
			{Samples: 7, EyeBuffer: 3, Redraw: 1, Error: 3.1},
			{Samples: 10, EyeBuffer: 4, Redraw: 2, Error: 1.8},
		},
		Periphery: [3]TierSpec{
			{Samples: 2, EyeBuffer: 1, Redraw: 1, Error: 9.0},
			{Samples: 4, EyeBuffer: 2, Redraw: 1, Error: 6.2},
			{Samples: 6, EyeBuffer: 3, Redraw: 1, Error: 4.0},
		},
	}
}

func (p Profile) Spec(region Region, tier Tier) TierSpec {
	if region == Center {
		return p.Center[tier]
	}
	return p.Periphery[tier]
}

func fallbackError(region Region) float64 {
	if region == Center {
		return 14.0
	}
	return 20.0
}

type JobKey struct {
	Eye    Eye    `json:"eye"`
	Region Region `json:"region"`
}

func JobKeys() [4]JobKey {
	return [4]JobKey{
		{Eye: Left, Region: Center},
		{Eye: Right, Region: Center},
		{Eye: Left, Region: Periphery},
		{Eye: Right, Region: Periphery},
	}
}

func (k JobKey) String() string {
	return fmt.Sprintf("%s/%s", k.Eye, k.Region)
}

type Budget struct {
	Samples   int    `json:"samples"`
	Redraw    int    `json:"redraw"`
	EyeBuffer [2]int `json:"eye_buffer"`
}

func (b Budget) EyeBufferTotal() int {
	return b.EyeBuffer[Left] + b.EyeBuffer[Right]
}

type Frame struct {
	Index        int    `json:"index"`
	Budget       Budget `json:"budget"`
	PredictionOK bool   `json:"prediction_ok"`
}

type Submission struct {
	Key  JobKey   `json:"key"`
	Tier Tier     `json:"tier"`
	Spec TierSpec `json:"spec"`
}

type FallbackFlags struct {
	Budget     bool `json:"budget"`
	EyeCap     bool `json:"eye_cap"`
	Prediction bool `json:"prediction"`
	Floor      bool `json:"floor"`
}

func (f FallbackFlags) Count() int {
	n := 0
	for _, set := range [...]bool{f.Budget, f.EyeCap, f.Prediction, f.Floor} {
		if set {
			n++
		}
	}
	return n
}

func (f FallbackFlags) Merge(other FallbackFlags) FallbackFlags {
	return FallbackFlags{
		Budget:     f.Budget || other.Budget,
		EyeCap:     f.EyeCap || other.EyeCap,
		Prediction: f.Prediction || other.Prediction,
		Floor:      f.Floor || other.Floor,
	}
}

func (f FallbackFlags) String() string {
	var parts []string
	if f.Budget {
		parts = append(parts, "budget")
	}
	if f.EyeCap {
		parts = append(parts, "eye-cap")
	}
	if f.Prediction {
		parts = append(parts, "prediction")
	}
	if f.Floor {
		parts = append(parts, "floor")
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, "+")
}

type FrameResult struct {
	Index           int           `json:"index"`
	PredictionOK    bool          `json:"prediction_ok"`
	PredictionHold  bool          `json:"prediction_hold"`
	FloorPreserved  bool          `json:"floor_preserved"`
	Submitted       []Submission  `json:"submitted"`
	OfferedTasks    int           `json:"offered_tasks"`
	SamplesUsed     int           `json:"samples_used"`
	RedrawUsed      int           `json:"redraw_used"`
	EyeBufferUsed   [2]int        `json:"eye_buffer_used"`
	CenterError     float64       `json:"center_error"`
	PeripheralError float64       `json:"peripheral_error"`
	Fallbacks       FallbackFlags `json:"fallbacks"`
}

func (r FrameResult) SubmittedTasks() int {
	return len(r.Submitted)
}

type FallbackTotals struct {
	Budget     int `json:"budget"`
	EyeCap     int `json:"eye_cap"`
	Prediction int `json:"prediction"`
	Floor      int `json:"floor"`
}

func (t FallbackTotals) Total() int {
	return t.Budget + t.EyeCap + t.Prediction + t.Floor
}

type Metrics struct {
	Scenario             string         `json:"scenario"`
	Policy               Policy         `json:"policy"`
	Frames               int            `json:"frames"`
	CenterError          float64        `json:"center_error"`
	PeripheralError      float64        `json:"peripheral_error"`
	SampleUse            float64        `json:"sample_use"`
	EyeBufferUse         float64        `json:"eye_buffer_use"`
	RedrawUse            float64        `json:"redraw_use"`
	OfferedTasks         int            `json:"offered_tasks"`
	SubmittedTasks       int            `json:"submitted_tasks"`
	CompletionRate       float64        `json:"completion_rate"`
	FloorPreservedFrames int            `json:"floor_preserved_frames"`
	FloorFailureFrames   int            `json:"floor_failure_frames"`
	Fallbacks            FallbackTotals `json:"fallbacks"`
}

type ScenarioReport struct {
	Scenario Scenario      `json:"scenario"`
	Metrics  Metrics       `json:"metrics"`
	Frames   []FrameResult `json:"frames"`
}
