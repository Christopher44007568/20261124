package preview

import (
	"errors"
	"fmt"
	"regexp"

	"headpose"
)

// Anchor is a world-space point to visualize and project into Screen.
type Anchor struct {
	Label    string
	Position headpose.Vec3
	Color    string
}

// CameraSample is one camera pose in World along the preview trajectory.
type CameraSample struct {
	Label         string
	CameraToWorld headpose.Transform
}

// Preview bundles the scene, anchors, and camera trajectory to render.
type Preview struct {
	Scene            *headpose.Scene
	Anchors          []Anchor
	CameraTrajectory []CameraSample
}

// Options controls SVG dimensions and the displayed screen viewport.
type Options struct {
	Title              string
	Width              int
	Height             int
	ScreenWidthPixels  int
	ScreenHeightPixels int
}

var (
	defaultAnchorColors = []string{"#ffb454", "#ff7b72", "#7ee787", "#d2a8ff", "#79c0ff"}
	hexColor            = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
)

type preparedPreview struct {
	scene      *headpose.Scene
	anchors    []Anchor
	trajectory []headpose.Transform
	labels     []string
	options    Options
}

func prepare(input Preview, options Options) (preparedPreview, error) {
	if input.Scene == nil {
		return preparedPreview{}, errors.New("preview scene is required")
	}
	if len(input.Anchors) == 0 {
		return preparedPreview{}, errors.New("at least one anchor is required")
	}
	if len(input.CameraTrajectory) < 2 {
		return preparedPreview{}, errors.New("camera trajectory needs at least two samples")
	}
	if options.Width == 0 {
		options.Width = 1180
	}
	if options.Height == 0 {
		options.Height = 720
	}
	if options.Width < 800 || options.Height < 560 {
		return preparedPreview{}, errors.New("preview must be at least 800x560")
	}
	if options.ScreenWidthPixels == 0 {
		options.ScreenWidthPixels = 1280
	}
	if options.ScreenHeightPixels == 0 {
		options.ScreenHeightPixels = 720
	}
	if options.ScreenWidthPixels <= 0 || options.ScreenHeightPixels <= 0 {
		return preparedPreview{}, errors.New("screen viewport must be positive")
	}
	if options.Title == "" {
		options.Title = "Head-pose coordinate frames"
	}

	anchors := make([]Anchor, len(input.Anchors))
	for i, anchor := range input.Anchors {
		if !anchor.Position.IsFinite() {
			return preparedPreview{}, fmt.Errorf("anchor %d has a non-finite position", i)
		}
		if anchor.Label == "" {
			anchor.Label = fmt.Sprintf("A%d", i+1)
		}
		if !hexColor.MatchString(anchor.Color) {
			anchor.Color = defaultAnchorColors[i%len(defaultAnchorColors)]
		}
		anchors[i] = anchor
	}

	trajectory := make([]headpose.Transform, len(input.CameraTrajectory))
	labels := make([]string, len(input.CameraTrajectory))
	for i, sample := range input.CameraTrajectory {
		if err := sample.CameraToWorld.Validate(); err != nil {
			return preparedPreview{}, fmt.Errorf("camera sample %d: %w", i, err)
		}
		if !sample.CameraToWorld.IsRigid(1e-9) {
			return preparedPreview{}, fmt.Errorf("camera sample %d must be rigid", i)
		}
		trajectory[i] = sample.CameraToWorld
		labels[i] = sample.Label
		if labels[i] == "" {
			labels[i] = fmt.Sprintf("camera sample %d", i+1)
		}
	}
	return preparedPreview{
		scene:      input.Scene,
		anchors:    anchors,
		trajectory: trajectory,
		labels:     labels,
		options:    options,
	}, nil
}
