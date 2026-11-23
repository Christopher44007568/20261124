package record

import (
	"encoding/json"
	"fmt"
	"image"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"

	"stereogaze/internal/gaze"
	"stereogaze/internal/render"
)

// FrameMetrics records sharing, overlap and invariant data for one frame.
type FrameMetrics struct {
	Frame              int        `json:"frame"`
	LeftOccluded       bool       `json:"leftOccluded"`
	RightOccluded      bool       `json:"rightOccluded"`
	LeftGaze           gaze.Point `json:"leftGaze"`
	RightGaze          gaze.Point `json:"rightGaze"`
	LeftFovealCells    int        `json:"leftFovealCells"`
	RightFovealCells   int        `json:"rightFovealCells"`
	LeftBoundaryCells  int        `json:"leftBoundaryCells"`
	RightBoundaryCells int        `json:"rightBoundaryCells"`
	OverlapCells       int        `json:"overlapCells"`
	SharedCells        int        `json:"sharedCells"`
	StandaloneWork     int        `json:"standaloneWork"`
	OptimizedWork      int        `json:"optimizedWork"`
	WorkSaved          int        `json:"workSaved"`
	InvariantHeld      bool       `json:"invariantHeld"`
}

// ScenarioMetrics is the full recording of one scenario.
type ScenarioMetrics struct {
	Scenario string         `json:"scenario"`
	GridSize int            `json:"gridSize"`
	Frames   []FrameMetrics `json:"frames"`
}

// Recorder writes frames, GIFs and structured metrics into an output root.
type Recorder struct {
	Root      string
	Delay100  int
	metrics   []ScenarioMetrics
	gifFrames map[string][]*image.Paletted
}

// NewRecorder creates the output directory and returns a recorder.
func NewRecorder(root string) (*Recorder, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("record: create output root: %w", err)
	}
	return &Recorder{Root: root, Delay100: 9}, nil
}

// BeginScenario starts a new metrics block for one scenario.
func (r *Recorder) BeginScenario(name string, gridSize int) *ScenarioMetrics {
	return &ScenarioMetrics{Scenario: name, GridSize: gridSize}
}

// CommitScenario stores the completed metrics block.
func (r *Recorder) CommitScenario(metrics *ScenarioMetrics) {
	for index := range r.metrics {
		if r.metrics[index].Scenario == metrics.Scenario {
			r.metrics[index] = *metrics
			return
		}
	}
	r.metrics = append(r.metrics, *metrics)
}

// AddFrame renders and writes one frame and verifies the sharing invariant.
func (r *Recorder) AddFrame(metrics *ScenarioMetrics, scenarioName string, frameNumber, gridSize int,
	left, right *gaze.Mask, report *gaze.Report) error {

	rebuiltLeft, rebuiltRight := gaze.ApplySharing(left, right, report)
	invariantHeld := left.Equals(rebuiltLeft) && right.Equals(rebuiltRight)
	if !invariantHeld {
		return fmt.Errorf("record: sharing changed a monocular mask in %s frame %d", scenarioName, frameNumber)
	}

	art := render.RenderFrame(left, right, report, frameNumber)
	frameDir := filepath.Join(r.Root, scenarioName)
	if err := os.MkdirAll(frameDir, 0o755); err != nil {
		return fmt.Errorf("record: create frame dir: %w", err)
	}
	pngPath := filepath.Join(frameDir, fmt.Sprintf("frame_%03d.png", frameNumber))
	if err := writePNG(pngPath, art); err != nil {
		return err
	}

	metrics.Frames = append(metrics.Frames, FrameMetrics{
		Frame:              frameNumber,
		LeftOccluded:       left.Occluded,
		RightOccluded:      right.Occluded,
		LeftGaze:           left.Gaze,
		RightGaze:          right.Gaze,
		LeftFovealCells:    left.CountLevel(gaze.Foveal),
		RightFovealCells:   right.CountLevel(gaze.Foveal),
		LeftBoundaryCells:  left.CountBoundary(),
		RightBoundaryCells: right.CountBoundary(),
		OverlapCells:       report.OverlapCount,
		SharedCells:        report.SharedCount,
		StandaloneWork:     report.StandaloneWork,
		OptimizedWork:      report.SharedWork,
		WorkSaved:          report.StandaloneWork - report.SharedWork,
		InvariantHeld:      invariantHeld,
	})

	if r.gifFrames == nil {
		r.gifFrames = make(map[string][]*image.Paletted)
	}
	r.gifFrames[scenarioName] = append(r.gifFrames[scenarioName], art)
	return nil
}

// Finish writes one looping GIF per scenario and the combined metrics file.
func (r *Recorder) Finish() error {
	for scenarioName, frames := range r.gifFrames {
		path := filepath.Join(r.Root, scenarioName+".gif")
		if err := writeGIF(path, render.AnimatedGIF(frames, r.Delay100)); err != nil {
			return err
		}
	}
	metricsPath := filepath.Join(r.Root, "records.json")
	data, err := json.MarshalIndent(r.metrics, "", "  ")
	if err != nil {
		return fmt.Errorf("record: encode metrics: %w", err)
	}
	if err := os.WriteFile(metricsPath, data, 0o644); err != nil {
		return fmt.Errorf("record: write metrics: %w", err)
	}
	return nil
}

func writePNG(path string, art *image.Paletted) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("record: create png: %w", err)
	}
	defer file.Close()
	if err := png.Encode(file, art); err != nil {
		return fmt.Errorf("record: encode png: %w", err)
	}
	return nil
}

func writeGIF(path string, animated *gif.GIF) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("record: create gif: %w", err)
	}
	defer file.Close()
	if err := gif.EncodeAll(file, animated); err != nil {
		return fmt.Errorf("record: encode gif: %w", err)
	}
	return nil
}
