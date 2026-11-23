package analyze

import (
	"fmt"
	"math"

	"posebench/internal/geom"
	"posebench/internal/motion"
	"posebench/internal/scan"
	"posebench/internal/strategy"
)

type Config struct {
	Frames        int
	Segments      int
	UnifiedPhase  float64
	JumpBudgetDeg float64
	JumpBudgetPx  float64
}

func DefaultConfig() Config {
	return Config{
		Frames:        120,
		Segments:      8,
		UnifiedPhase:  0.5,
		JumpBudgetDeg: 0.5,
		JumpBudgetPx:  0.5,
	}
}

type StrategySummary struct {
	Kind                string       `json:"kind"`
	Name                string       `json:"name"`
	Deployable          bool         `json:"deployable"`
	Note                string       `json:"note"`
	PosesPerFrame       int          `json:"poses_per_frame"`
	ErrorDeg            Distribution `json:"orientation_error_deg"`
	ErrorMM             Distribution `json:"translation_error_mm"`
	MaxRowStepDeg       float64      `json:"max_adjacent_row_step_deg"`
	MaxRowStepPx        float64      `json:"max_adjacent_row_step_px"`
	Boundaries          int          `json:"boundary_count"`
	BoundaryRows        []int        `json:"boundary_rows"`
	BoundaryStepDeg     Distribution `json:"boundary_step_deg"`
	BoundaryStepPx      Distribution `json:"boundary_step_px"`
	BoundaryResidualDeg float64      `json:"max_boundary_discontinuity_deg"`
	BoundaryResidualPx  float64      `json:"max_boundary_discontinuity_px"`
	BoundaryOK          bool         `json:"boundary_within_budget"`
}

type ScenarioReport struct {
	Name                      string            `json:"name"`
	Description               string            `json:"description"`
	PeriodSec                 float64           `json:"period_sec"`
	PeakAngularSpeedDegPerSec float64           `json:"peak_angular_speed_deg_per_sec"`
	Frames                    int               `json:"frames"`
	Shear                     ShearSummary      `json:"image_shear"`
	Strategies                []StrategySummary `json:"strategies"`
}

type Report struct {
	Schema          string           `json:"schema"`
	Camera          CameraInfo       `json:"camera"`
	ReadoutMS       float64          `json:"readout_ms"`
	FramePeriodMS   float64          `json:"frame_period_ms"`
	Segments        int              `json:"segments"`
	FramesPerPeriod int              `json:"frames_per_period"`
	JumpBudgetDeg   float64          `json:"boundary_budget_deg"`
	JumpBudgetPx    float64          `json:"boundary_budget_px"`
	Scenarios       []ScenarioReport `json:"scenarios"`
}

type CameraInfo struct {
	Width  int     `json:"width"`
	Height int     `json:"height"`
	FX     float64 `json:"fx"`
	FY     float64 `json:"fy"`
	CX     float64 `json:"cx"`
	CY     float64 `json:"cy"`
}

func Run(profiles []motion.Profile, sh scan.Shutter, cfg Config) (Report, error) {
	if cfg.Frames <= 0 {
		return Report{}, fmt.Errorf("frames must be positive")
	}
	report := Report{
		Schema:          "posebench/v1",
		Camera:          cameraInfo(sh.Camera),
		ReadoutMS:       sh.ReadoutSec * 1000,
		FramePeriodMS:   sh.FrameSec * 1000,
		Segments:        cfg.Segments,
		FramesPerPeriod: cfg.Frames,
		JumpBudgetDeg:   cfg.JumpBudgetDeg,
		JumpBudgetPx:    cfg.JumpBudgetPx,
	}
	for _, profile := range profiles {
		scenario, err := RunScenario(profile, sh, cfg)
		if err != nil {
			return Report{}, err
		}
		report.Scenarios = append(report.Scenarios, scenario)
	}
	return report, nil
}

type accumulator struct {
	sampling   strategy.Sampling
	errDeg     []float64
	errMM      []float64
	bStepDeg   []float64
	bStepPx    []float64
	maxStepDeg float64
	maxStepPx  float64
	maxResDeg  float64
	maxResPx   float64
}

func RunScenario(profile motion.Profile, sh scan.Shutter, cfg Config) (ScenarioReport, error) {
	if cfg.Frames <= 0 {
		return ScenarioReport{}, fmt.Errorf("frames must be positive")
	}
	kinds := []strategy.Kind{
		strategy.UniformStart,
		strategy.UniformMid,
		strategy.PerRow,
		strategy.Segmented,
		strategy.HardSegments,
	}
	accs := make([]accumulator, len(kinds))
	shearFrames := make([]ShearFrame, 0, cfg.Frames)
	center := scan.FaceCenter()

	for frameIdx := 0; frameIdx < cfg.Frames; frameIdx++ {
		frameStart := profile.PeriodSec * float64(frameIdx) / float64(cfg.Frames)
		shearFrames = append(shearFrames, MeasureShear(profile, sh, frameStart))
		truePoses := make([]geom.Pose, sh.Camera.Height)
		for row := range truePoses {
			truePoses[row] = profile.PoseAt(sh.RowTime(frameStart, row))
		}

		for i, kind := range kinds {
			sampling, err := strategy.Build(kind, strategy.Config{
				Segments:     cfg.Segments,
				UnifiedPhase: cfg.UnifiedPhase,
			}, sh, frameStart, profile)
			if err != nil {
				return ScenarioReport{}, err
			}
			acc := &accs[i]
			acc.sampling = sampling
			for row := 0; row < sh.Camera.Height; row++ {
				acc.errDeg = append(acc.errDeg, geom.AngularErrorDeg(sampling.Poses[row], truePoses[row]))
				acc.errMM = append(acc.errMM, geom.TranslationErrorMM(sampling.Poses[row], truePoses[row]))
				if row == 0 {
					continue
				}
				stepDeg := geom.AngularErrorDeg(sampling.Poses[row-1], sampling.Poses[row])
				stepPx := projectedStep(sh.Camera, sampling.Poses[row-1], sampling.Poses[row], center)
				acc.maxStepDeg = maxFloat(acc.maxStepDeg, stepDeg)
				acc.maxStepPx = maxFloat(acc.maxStepPx, stepPx)
			}
			for _, boundary := range sampling.SegmentBoundaries {
				if boundary < 1 || boundary >= sh.Camera.Height {
					continue
				}
				prev, next := boundary-1, boundary
				stepDeg := geom.AngularErrorDeg(sampling.Poses[prev], sampling.Poses[next])
				stepPx := projectedStep(sh.Camera, sampling.Poses[prev], sampling.Poses[next], center)
				trueDeg := geom.AngularErrorDeg(truePoses[prev], truePoses[next])
				truePx := projectedStep(sh.Camera, truePoses[prev], truePoses[next], center)
				acc.bStepDeg = append(acc.bStepDeg, stepDeg)
				acc.bStepPx = append(acc.bStepPx, stepPx)
				acc.maxResDeg = maxFloat(acc.maxResDeg, maxFloat(0, stepDeg-trueDeg))
				acc.maxResPx = maxFloat(acc.maxResPx, maxFloat(0, stepPx-truePx))
			}
		}
	}

	scenario := ScenarioReport{
		Name:                      profile.Name,
		Description:               profile.Description,
		PeriodSec:                 profile.PeriodSec,
		PeakAngularSpeedDegPerSec: round3(profile.PeakAngularSpeedDegPerSec()),
		Frames:                    cfg.Frames,
		Shear:                     SummarizeShear(shearFrames),
	}
	for i := range accs {
		acc := &accs[i]
		hasBoundaries := len(acc.bStepDeg) > 0
		boundaryRows := append([]int{}, acc.sampling.SegmentBoundaries...)
		summary := StrategySummary{
			Kind:                string(acc.sampling.Kind),
			Name:                acc.sampling.Name,
			Deployable:          acc.sampling.Deployable,
			Note:                acc.sampling.Note,
			PosesPerFrame:       acc.sampling.PoseCount,
			ErrorDeg:            Summarize(acc.errDeg),
			ErrorMM:             Summarize(acc.errMM),
			MaxRowStepDeg:       acc.maxStepDeg,
			MaxRowStepPx:        acc.maxStepPx,
			Boundaries:          len(acc.sampling.SegmentBoundaries),
			BoundaryRows:        boundaryRows,
			BoundaryStepDeg:     Summarize(acc.bStepDeg),
			BoundaryStepPx:      Summarize(acc.bStepPx),
			BoundaryResidualDeg: acc.maxResDeg,
			BoundaryResidualPx:  acc.maxResPx,
			BoundaryOK:          !hasBoundaries || (acc.maxResDeg <= cfg.JumpBudgetDeg && acc.maxResPx <= cfg.JumpBudgetPx),
		}
		scenario.Strategies = append(scenario.Strategies, summary)
	}
	return scenario, nil
}

func projectedStep(cam scan.Camera, a, b geom.Pose, local geom.Vec3) float64 {
	pa := cam.Project(a, local)
	pb := cam.Project(b, local)
	if !pa.Valid || !pb.Valid {
		return math.NaN()
	}
	return math.Hypot(pb.X-pa.X, pb.Y-pa.Y)
}

func cameraInfo(cam scan.Camera) CameraInfo {
	return CameraInfo{
		Width: cam.Width, Height: cam.Height,
		FX: cam.FX, FY: cam.FY, CX: cam.CX, CY: cam.CY,
	}
}

func round3(v float64) float64 {
	return math.Round(v*1000) / 1000
}
