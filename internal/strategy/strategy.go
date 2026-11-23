package strategy

import (
	"fmt"
	"math"

	"posebench/internal/geom"
	"posebench/internal/motion"
	"posebench/internal/scan"
)

type Kind string

const (
	UniformStart Kind = "uniform-start"
	UniformMid   Kind = "uniform-mid"
	PerRow       Kind = "per-row"
	Segmented    Kind = "segmented-smooth"
	HardSegments Kind = "segmented-hard"
)

type Sampling struct {
	Kind              Kind
	Name              string
	Poses             []geom.Pose
	PoseCount         int
	SegmentBoundaries []int
	Deployable        bool
	Note              string
}

type Config struct {
	Segments     int
	UnifiedPhase float64
}

func DefaultConfig() Config {
	return Config{Segments: 8, UnifiedPhase: 0.5}
}

// Build samples one pose per image row for the requested strategy.
func Build(kind Kind, cfg Config, sh scan.Shutter, frameStart float64, profile motion.Profile) (Sampling, error) {
	h := sh.Camera.Height
	if h < 1 {
		return Sampling{}, fmt.Errorf("camera height must be positive")
	}
	poses := make([]geom.Pose, h)

	switch kind {
	case UniformStart:
		q := profile.PoseAt(frameStart)
		fill(poses, q)
		return Sampling{
			Kind:       kind,
			Name:       "uniform @ frame start",
			Poses:      poses,
			PoseCount:  1,
			Deployable: false,
			Note:       "diagnostic only: a frame-start pose must not represent the whole panel",
		}, nil

	case UniformMid:
		phase := cfg.UnifiedPhase
		if math.IsNaN(phase) {
			phase = 0.5
		}
		phase = clamp(phase, 0, 1)
		if phase == 0 {
			return Sampling{}, fmt.Errorf("uniform-mid cannot use the frame-start pose; use the diagnostic uniform-start baseline explicitly")
		}
		q := profile.PoseAt(frameStart + phase*sh.ReadoutSec)
		fill(poses, q)
		return Sampling{
			Kind:       kind,
			Name:       fmt.Sprintf("uniform @ %.0f%% readout", phase*100),
			Poses:      poses,
			PoseCount:  1,
			Deployable: true,
			Note:       "one timestamp for the entire panel",
		}, nil

	case PerRow:
		for row := 0; row < h; row++ {
			poses[row] = profile.PoseAt(sh.RowTime(frameStart, row))
		}
		return Sampling{
			Kind:       kind,
			Name:       "per-row",
			Poses:      poses,
			PoseCount:  h,
			Deployable: true,
			Note:       "independent pose at every row timestamp",
		}, nil

	case Segmented, HardSegments:
		n := cfg.Segments
		if n < 1 {
			return Sampling{}, fmt.Errorf("segments must be >= 1, got %d", n)
		}
		if n > h {
			return Sampling{}, fmt.Errorf("segments (%d) exceeds row count (%d)", n, h)
		}
		centres := segmentCentres(h, n)
		anchors := make([]geom.Pose, n)
		for i, centre := range centres {
			anchors[i] = profile.PoseAt(sh.RowTimeAtY(frameStart, centre))
		}
		if kind == HardSegments {
			buildHard(poses, anchors, h)
		} else {
			buildSmooth(poses, anchors, centres, h)
		}
		boundaries := make([]int, 0, n-1)
		for i := 1; i < n; i++ {
			row := (i*h + n - 1) / n
			if row >= 1 && row < h {
				boundaries = append(boundaries, row)
			}
		}
		name := "segmented smooth"
		deployable := true
		note := "slerp between segment-centre anchors with a C1 smoothstep at boundaries"
		if kind == HardSegments {
			name = "segmented hard"
			deployable = false
			note = "diagnostic only: piecewise-constant anchors jump at boundaries"
		}
		return Sampling{
			Kind:              kind,
			Name:              name,
			Poses:             poses,
			PoseCount:         n,
			SegmentBoundaries: boundaries,
			Deployable:        deployable,
			Note:              note,
		}, nil
	}

	return Sampling{}, fmt.Errorf("unknown strategy %q", kind)
}

func segmentCentres(height, segments int) []float64 {
	h := float64(height - 1)
	centres := make([]float64, segments)
	for i := range centres {
		centres[i] = (float64(i) + 0.5) * h / float64(segments)
	}
	return centres
}

func buildHard(poses []geom.Pose, anchors []geom.Pose, height int) {
	n := len(anchors)
	for row := 0; row < height; row++ {
		idx := row * n / height
		if idx >= n {
			idx = n - 1
		}
		poses[row] = anchors[idx]
	}
}

func buildSmooth(poses []geom.Pose, anchors []geom.Pose, centres []float64, height int) {
	n := len(anchors)
	if n == 1 {
		fill(poses, anchors[0])
		return
	}
	for row := 0; row < height; row++ {
		y := float64(row)
		switch {
		case y <= centres[0]:
			poses[row] = anchors[0]
		case y >= centres[n-1]:
			poses[row] = anchors[n-1]
		default:
			i := 0
			for i+1 < n-1 && y >= centres[i+1] {
				i++
			}
			u := (y - centres[i]) / (centres[i+1] - centres[i])
			s := u * u * (3 - 2*u)
			poses[row] = geom.InterpolatePose(anchors[i], anchors[i+1], s)
		}
	}
}

func fill(poses []geom.Pose, p geom.Pose) {
	for i := range poses {
		poses[i] = p
	}
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
