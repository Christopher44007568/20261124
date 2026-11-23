package compositor

import (
	"encoding/json"
	"fmt"
)

// Eye identifies the view whose depth data is being evaluated.
type Eye int

const (
	LeftEye Eye = iota
	RightEye
)

func (e Eye) String() string {
	switch e {
	case LeftEye:
		return "left"
	case RightEye:
		return "right"
	default:
		return fmt.Sprintf("eye(%d)", int(e))
	}
}

func (e Eye) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.String())
}

// DepthMode describes how much depth information a submission carries.
type DepthMode string

const (
	DepthNone   DepthMode = "none"
	DepthSparse DepthMode = "sparse"
	DepthFull   DepthMode = "full"
)

// SurfaceKind is used for reporting; it never changes the occlusion policy.
type SurfaceKind string

const (
	SurfaceHand         SurfaceKind = "hand"
	SurfacePanel        SurfaceKind = "panel"
	SurfaceNotification SurfaceKind = "notification"
)

// Layer is one covering surface. Order is an explicit back-to-front fallback:
// larger values are drawn later. It is the only ordering fallback used by the
// compositor; System never receives implicit priority.
type Layer struct {
	ID       string
	Kind     SurfaceKind
	System   bool
	Order    int
	Coverage []bool
	Left     []float64
	Right    []float64
}

func (l Layer) Depths(eye Eye) []float64 {
	if eye == RightEye {
		return l.Right
	}
	return l.Left
}

// Scene holds the raw, unquantized truth used to score submissions.
type Scene struct {
	Name   string
	Width  int
	Height int
	Layers []Layer
}

func (s Scene) Pixels() int {
	return s.Width * s.Height
}

// Submission is a complete candidate composition. Its layers use the same IDs
// as the scene, but carry the depth values actually submitted for each eye.
type Submission struct {
	Name    string
	Mode    DepthMode
	Quantum float64
	Layers  []Layer
}

// Policy controls only diagnostic negative controls. SafePolicy is the
// production behavior.
type Policy struct {
	Name              string
	SystemAlwaysFront bool
	IgnoreIncomplete  bool
	LeftDepthForBoth  bool
}

var SafePolicy = Policy{Name: "per-eye-safe"}

var SystemFrontPolicy = Policy{
	Name:              "negative-system-always-front",
	SystemAlwaysFront: true,
}

var IgnoreIncompletePolicy = Policy{
	Name:             "negative-ignore-incomplete-depth",
	IgnoreIncomplete: true,
}

var LeftEyeForBothPolicy = Policy{
	Name:             "negative-left-depth-for-right-eye",
	LeftDepthForBoth: true,
}

type DecisionReason string

const (
	ReasonBackground      DecisionReason = "background"
	ReasonNoDepth         DecisionReason = "painter-order-no-depth"
	ReasonIncompleteDepth DecisionReason = "painter-order-incomplete-depth"
	ReasonNoValidDepth    DecisionReason = "painter-order-no-valid-depth"
	ReasonNearestDepth    DecisionReason = "nearest-valid-depth"
	ReasonQuantizationTie DecisionReason = "nearest-depth-quantization-ambiguity"
	ReasonSystemFront     DecisionReason = "system-always-front"
)

type CandidateStatus string

const (
	StatusSelected CandidateStatus = "selected"
	StatusOccluded CandidateStatus = "occluded"
	StatusMissing  CandidateStatus = "missing-depth"
	StatusInvalid  CandidateStatus = "invalid-depth"
)

// CandidateDecision records what the compositor observed for one layer at one
// pixel. Depth is zero when DepthValid is false.
type CandidateDecision struct {
	LayerID    string          `json:"layerId"`
	Kind       SurfaceKind     `json:"kind"`
	System     bool            `json:"system"`
	Order      int             `json:"order"`
	Depth      float64         `json:"depthMeters"`
	DepthValid bool            `json:"depthValid"`
	SourceEye  Eye             `json:"sourceEye"`
	Selected   bool            `json:"selected"`
	Status     CandidateStatus `json:"status"`
}

// PixelDecision is a complete coverage/occlusion record for one eye and pixel.
type PixelDecision struct {
	X          int                 `json:"x"`
	Y          int                 `json:"y"`
	Eye        Eye                 `json:"eye"`
	Winner     string              `json:"winner"`
	Reason     DecisionReason      `json:"reason"`
	Ambiguous  bool                `json:"ambiguous"`
	Candidates []CandidateDecision `json:"candidates"`
}

type ErrorKind string

const (
	ErrorWrongFront      ErrorKind = "wrong-front-surface"
	ErrorMissingSurface  ErrorKind = "missing-surface"
	ErrorUnexpectedLayer ErrorKind = "unexpected-surface"
)

// OcclusionError compares the submitted result with the unquantized scene.
type OcclusionError struct {
	X        int       `json:"x"`
	Y        int       `json:"y"`
	Eye      Eye       `json:"eye"`
	Expected string    `json:"expected"`
	Actual   string    `json:"actual"`
	Kind     ErrorKind `json:"kind"`
	Detail   string    `json:"detail"`
}

type Summary struct {
	Pixels                int `json:"pixels"`
	CoveredPixels         int `json:"coveredPixels"`
	CorrectPixels         int `json:"correctPixels"`
	WrongOcclusions       int `json:"wrongOcclusions"`
	OrderFallbacks        int `json:"orderFallbacks"`
	IncompleteDepthPixels int `json:"incompleteDepthPixels"`
	InvalidDepthSamples   int `json:"invalidDepthSamples"`
	QuantizationAmbiguous int `json:"quantizationAmbiguous"`
}

type EyeResult struct {
	Eye       Eye              `json:"eye"`
	Summary   Summary          `json:"summary"`
	Decisions []PixelDecision  `json:"decisions"`
	Errors    []OcclusionError `json:"errors"`
}

type Result struct {
	Submission string      `json:"submission"`
	Mode       DepthMode   `json:"mode"`
	Policy     string      `json:"policy"`
	Width      int         `json:"width"`
	Height     int         `json:"height"`
	Eyes       []EyeResult `json:"eyes"`
	Summary    Summary     `json:"summary"`
	layers     []Layer
}

type Comparison struct {
	Scene       Scene        `json:"scene"`
	Submissions []Result     `json:"submissions"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// Diagnostic is a focused edge-case result, optionally with a negative control.
type Diagnostic struct {
	Name   string  `json:"name"`
	Detail string  `json:"detail"`
	Safe   Result  `json:"safe"`
	Bad    *Result `json:"bad,omitempty"`
}
