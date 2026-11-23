package rollingshutter

import (
	"errors"
	"sort"
	"time"
)

const (
	FlagReadoutDirectionReversed = "readout_direction_reversed"
	FlagPoseSamplesMissing       = "pose_samples_missing"
	FlagPoseSamplingErrors       = "pose_sampling_errors"
	FlagLineProjectionIncomplete = "line_projection_incomplete"
)

// AnalysisConfig combines the independent clocks, geometry and pose source.
type AnalysisConfig struct {
	Timing     RowTiming
	Camera     Camera
	Line       LineSegment
	PoseSource PoseSource
}

// RowProjection is the complete result for one physical image row. A missing
// pose leaves ProjectionError set and omits Image/World/Pose.
type RowProjection struct {
	Row             int   `json:"row"`
	ReadoutOrder    int   `json:"readoutOrder"`
	ExposureStartNS int64 `json:"exposureStartNs"`
	ExposureEndNS   int64 `json:"exposureEndNs"`
	PoseSampleNS    int64 `json:"poseSampleNs"`
	TimeErrorNS     int64 `json:"timeErrorNs"`

	DirectionReversed bool    `json:"directionReversed"`
	PoseAvailable     bool    `json:"poseAvailable"`
	PoseMissing       bool    `json:"poseMissing"`
	Pose              *Pose   `json:"pose,omitempty"`
	Image             *Vec2   `json:"image,omitempty"`
	World             *Vec3   `json:"world,omitempty"`
	ProjectionError   string  `json:"projectionError,omitempty"`
	Parameter         float64 `json:"parameter,omitempty"`
}

type AnalysisSummary struct {
	ReadoutDirection  string `json:"readoutDirection"`
	DirectionReversed bool   `json:"directionReversed"`

	RowCount              int `json:"rowCount"`
	ProjectedRows         int `json:"projectedRows"`
	PoseMissingRows       int `json:"poseMissingRows"`
	PoseSamplingErrorRows int `json:"poseSamplingErrorRows"`
	ProjectionFailedRows  int `json:"projectionFailedRows"`

	GlobalShutterReferenceNS int64 `json:"globalShutterReferenceNs"`
	TopRowSampleNS           int64 `json:"topRowSampleNs"`
	BottomRowSampleNS        int64 `json:"bottomRowSampleNs"`
	TopBottomSampleDeltaNS   int64 `json:"topBottomSampleDeltaNs"`
	MaxAbsTimeErrorNS        int64 `json:"maxAbsTimeErrorNs"`
	MaxAbsTimeErrorRow       int   `json:"maxAbsTimeErrorRow"`

	StraightLineCurvature CurvatureResult `json:"straightLineCurvature"`
	Flags                 []string        `json:"flags"`
}

type Analysis struct {
	Summary AnalysisSummary `json:"summary"`
	Rows    []RowProjection `json:"rows"`
}

// Analyze creates one independent projection per row. No pose is reused
// between the top and bottom of the frame.
func Analyze(cfg AnalysisConfig) (Analysis, error) {
	if cfg.PoseSource == nil {
		return Analysis{}, errors.New("pose source is required")
	}
	if err := cfg.Timing.Validate(); err != nil {
		return Analysis{}, err
	}
	if err := cfg.Camera.Validate(); err != nil {
		return Analysis{}, err
	}
	if cfg.Line.A == cfg.Line.B {
		return Analysis{}, errors.New("line endpoints must differ")
	}
	height := cfg.Camera.Height
	rows := make([]RowProjection, 0, height)
	points := make([]ProjectedPoint, 0, height)
	flags := map[string]bool{}
	summary := AnalysisSummary{
		ReadoutDirection:  cfg.Timing.Config().Direction.String(),
		DirectionReversed: cfg.Timing.Config().Direction.Reversed(),
		RowCount:          height,
	}
	if summary.DirectionReversed {
		flags[FlagReadoutDirectionReversed] = true
	}

	maxAbsError := int64(-1)
	var reference time.Duration
	for row := 0; row < height; row++ {
		window, err := cfg.Timing.Row(row, height)
		if err != nil {
			return Analysis{}, err
		}
		if row == 0 {
			reference = cfg.Timing.GlobalShutterReference()
		}
		if row == 0 {
			summary.TopRowSampleNS = int64(window.PoseSampleTime)
		}
		if row == height-1 {
			summary.BottomRowSampleNS = int64(window.PoseSampleTime)
		}

		timeError := cfg.Timing.SignedTimeError(window)
		output := RowProjection{
			Row:               row,
			ReadoutOrder:      window.ReadoutOrder,
			ExposureStartNS:   int64(window.ExposureStart),
			ExposureEndNS:     int64(window.ExposureEnd),
			PoseSampleNS:      int64(window.PoseSampleTime),
			TimeErrorNS:       int64(timeError),
			DirectionReversed: window.DirectionReversed,
		}
		absoluteError := int64(timeError)
		if absoluteError < 0 {
			absoluteError = -absoluteError
		}
		if absoluteError > maxAbsError {
			maxAbsError = absoluteError
			summary.MaxAbsTimeErrorRow = row
		}

		pose, poseErr := cfg.PoseSource.PoseAt(window.PoseSampleTime)
		if poseErr != nil {
			output.ProjectionError = poseErr.Error()
			if errors.Is(poseErr, ErrPoseMissing) {
				output.PoseMissing = true
				summary.PoseMissingRows++
				flags[FlagPoseSamplesMissing] = true
			} else {
				summary.PoseSamplingErrorRows++
				flags[FlagPoseSamplingErrors] = true
			}
			rows = append(rows, output)
			continue
		}
		output.PoseAvailable = true
		poseCopy := pose
		output.Pose = &poseCopy

		projection, projectErr := ProjectLineAtRow(cfg.Camera, pose, cfg.Line, row)
		if projectErr != nil {
			output.ProjectionError = projectErr.Error()
			summary.ProjectionFailedRows++
			flags[FlagLineProjectionIncomplete] = true
			rows = append(rows, output)
			continue
		}
		imageCopy, worldCopy := projection.Image, projection.World
		output.Image = &imageCopy
		output.World = &worldCopy
		output.Parameter = projection.Parameter
		rows = append(rows, output)
		points = append(points, ProjectedPoint{
			Row: row,
			X:   projection.Image.X,
			Y:   projection.Image.Y,
		})
	}

	summary.ProjectedRows = len(points)
	summary.GlobalShutterReferenceNS = int64(reference)
	summary.TopBottomSampleDeltaNS = summary.BottomRowSampleNS - summary.TopRowSampleNS
	if maxAbsError < 0 {
		maxAbsError = 0
	}
	summary.MaxAbsTimeErrorNS = maxAbsError
	summary.StraightLineCurvature = StraightLineCurvature(points)
	summary.Flags = sortedFlags(flags)
	return Analysis{Summary: summary, Rows: rows}, nil
}

func sortedFlags(flags map[string]bool) []string {
	result := make([]string, 0, len(flags))
	for flag := range flags {
		result = append(result, flag)
	}
	sort.Strings(result)
	return result
}
