package reprojlab

import (
	"fmt"
)

type ReportConfig struct {
	Width       int         `json:"width"`
	Height      int         `json:"height"`
	Intrinsics  Intrinsics  `json:"intrinsics"`
	Distortion  Distortion  `json:"distortion"`
	Translation Translation `json:"translation"`
	LowResScale int         `json:"low_res_scale"`
}

type StatsReport struct {
	Passes        int     `json:"passes"`
	SourceSamples int64   `json:"source_samples"`
	Writes        int64   `json:"writes"`
	Offscreen     int64   `json:"offscreen"`
	Occluded      int64   `json:"occluded"`
	Invalid       int64   `json:"invalid"`
	InputHoles    int64   `json:"input_holes"`
	OutputPixels  int     `json:"output_pixels"`
	Holes         int     `json:"holes"`
	HolePercent   float64 `json:"hole_percent"`
}

type CompareReport struct {
	CommonPixels  int     `json:"common_pixels"`
	MissingPixels int     `json:"missing_pixels"`
	ExtraPixels   int     `json:"extra_pixels"`
	ColorRMSE     float64 `json:"color_rmse"`
	DepthRMSE     float64 `json:"depth_rmse"`
	MaxDepthError float64 `json:"max_depth_error"`
}

type ProbeReport struct {
	Name          string   `json:"name"`
	Note          string   `json:"note"`
	SourceDepth   float64  `json:"source_depth"`
	OutputX       float64  `json:"output_x"`
	OutputY       float64  `json:"output_y"`
	ExpectedDepth float64  `json:"expected_depth"`
	ActualDepth   *float64 `json:"actual_depth"`
	Filled        bool     `json:"filled"`
	DepthError    *float64 `json:"depth_error"`
}

type StageReport struct {
	Name  string      `json:"name"`
	Stats StatsReport `json:"stats"`
}

type OrderReport struct {
	Name          string        `json:"name"`
	Order         Order         `json:"order"`
	Variant       string        `json:"variant,omitempty"`
	Selected      bool          `json:"selected"`
	Stats         StatsReport   `json:"stats"`
	Stages        []StageReport `json:"stages"`
	Reference     CompareReport `json:"reference_compare"`
	ProbeFailures int           `json:"probe_failures"`
	Probes        []ProbeReport `json:"probes"`
}

type ChannelReport struct {
	Holes       [3]int         `json:"holes"`
	UnionHoles  int            `json:"union_holes"`
	Stats       [3]StatsReport `json:"stats"`
	EdgeShift   float64        `json:"right_edge_blue_shift_px"`
	CenterShift float64        `json:"center_blue_shift_px"`
}

type VariantReport struct {
	Result        OrderReport   `json:"result"`
	Reference     CompareReport `json:"reference_compare"`
	ProbeFailures int           `json:"probe_failures"`
}

type WrongDepthReport struct {
	Result         OrderReport   `json:"result"`
	CorrectB       CompareReport `json:"compare_to_correct_b"`
	ProbeFailures  int           `json:"probe_failures"`
	MeanProbeError *float64      `json:"mean_probe_depth_error"`
}

type ExperimentReport struct {
	Scene          string           `json:"scene"`
	Config         ReportConfig     `json:"config"`
	Objects        []string         `json:"objects"`
	SelectedOrder  Order            `json:"selected_order"`
	DecisionReason string           `json:"decision_reason"`
	Orders         []OrderReport    `json:"orders"`
	LowResolution  VariantReport    `json:"low_resolution"`
	PerChannel     ChannelReport    `json:"per_channel"`
	WrongDepth     WrongDepthReport `json:"wrong_depth_reuse"`
}

func BuildExperimentReport(cfg Config, maxProbeDepthError float64) ExperimentReport {
	scene := NewScene(cfg)
	decision := DecideOrder(scene, cfg, maxProbeDepthError)
	low := RunLowResDistortThenReproject(scene, cfg)
	ch := RunPerChannelDistortion(scene, cfg)
	wrong := RunWrongDepthReuse(scene, cfg)
	correctB := decision.B.Result

	objects := make([]string, len(scene.Objects))
	for i, o := range scene.Objects {
		objects[i] = o.String()
	}
	rep := ExperimentReport{
		Scene: scene.Name,
		Config: ReportConfig{
			Width: cfg.Width, Height: cfg.Height, Intrinsics: cfg.Intrinsics,
			Distortion: cfg.Distortion, Translation: cfg.Translation, LowResScale: cfg.LowResScale,
		},
		Objects:        objects,
		SelectedOrder:  decision.Selected,
		DecisionReason: decision.Reason,
		Orders: []OrderReport{
			orderReport(decision.A, decision.Selected == ReprojectThenDistort),
			orderReport(decision.B, decision.Selected == DistortThenReproject),
		},
		LowResolution: VariantReport{
			Result:    orderReport(OrderMetrics{Result: low, ReferenceDiff: CompareRasters(low.Final, decision.Reference), ProbeFailures: ProbeFailures(low.Probes, maxProbeDepthError)}, false),
			Reference: compareReport(CompareRasters(low.Final, decision.Reference)), ProbeFailures: ProbeFailures(low.Probes, maxProbeDepthError),
		},
		PerChannel: ChannelReport{
			Holes: ch.Holes, UnionHoles: ch.UnionHoles(),
			Stats:       [3]StatsReport{statsReport(ch.Stats[0], ch.Channels[0]), statsReport(ch.Stats[1], ch.Channels[1]), statsReport(ch.Stats[2], ch.Channels[2])},
			EdgeShift:   ChannelDisplacement(Point{X: 470, Y: 121}, cfg, 1.04),
			CenterShift: ChannelDisplacement(Point{X: cfg.Intrinsics.CX, Y: cfg.Intrinsics.CY}, cfg, 1.04),
		},
		WrongDepth: WrongDepthReport{
			Result:         orderReport(OrderMetrics{Result: wrong, ReferenceDiff: CompareRasters(wrong.Final, decision.Reference), ProbeFailures: ProbeFailures(wrong.Probes, maxProbeDepthError)}, false),
			CorrectB:       compareReport(CompareRasters(wrong.Final, correctB.Final)),
			ProbeFailures:  ProbeFailures(wrong.Probes, maxProbeDepthError),
			MeanProbeError: floatPtr(MeanProbeDepthError(wrong.Probes)),
		},
	}
	return rep
}

func orderReport(m OrderMetrics, selected bool) OrderReport {
	r := m.Result
	stages := make([]StageReport, len(r.Stages))
	for i, s := range r.Stages {
		stages[i] = StageReport{Name: s.Name, Stats: statsReport(s.Stats, nil)}
	}
	return OrderReport{
		Name: r.Name, Order: r.Order, Variant: r.Variant, Selected: selected,
		Stats: statsReport(r.Stats, r.Final), Stages: stages, Reference: compareReport(m.ReferenceDiff),
		ProbeFailures: m.ProbeFailures, Probes: probeReports(r.Probes),
	}
}

func compareReport(d RasterDiff) CompareReport {
	return CompareReport{
		CommonPixels: d.CommonPixels, MissingPixels: d.MissingPixels, ExtraPixels: d.ExtraPixels,
		ColorRMSE: d.ColorRMSE, DepthRMSE: d.DepthRMSE, MaxDepthError: d.MaxDepthError,
	}
}

func statsReport(s WarpStats, final *Raster) StatsReport {
	holes := s.OutputHoles
	pixels := s.OutputPixels
	if final != nil {
		holes = final.HoleCount()
		pixels = final.W * final.H
	}
	pct := 0.0
	if pixels > 0 {
		pct = 100 * float64(holes) / float64(pixels)
	}
	return StatsReport{
		Passes: s.Passes, SourceSamples: s.SourceSamples, Writes: s.Writes,
		Offscreen: s.Offscreen, Occluded: s.Occluded, Invalid: s.Invalid,
		InputHoles: s.InputHoles, OutputPixels: pixels, Holes: holes, HolePercent: pct,
	}
}

func probeReports(probes []ProbeResult) []ProbeReport {
	out := make([]ProbeReport, len(probes))
	for i, p := range probes {
		out[i] = ProbeReport{
			Name: p.Probe, Note: p.Note, SourceDepth: p.SourceDepth,
			OutputX: p.ExpectedPoint.X, OutputY: p.ExpectedPoint.Y,
			ExpectedDepth: p.ExpectedDepth, Filled: p.Filled,
		}
		if p.Filled {
			out[i].ActualDepth = floatPtr(p.ActualDepth)
		}
		if !isBadFloat(p.DepthError) {
			out[i].DepthError = floatPtr(p.DepthError)
		}
	}
	return out
}

func floatPtr(v float64) *float64 { return &v }

func isBadFloat(v float64) bool {
	return v != v || v > 1e300 || v < -1e300
}

func (r ExperimentReport) String() string {
	return fmt.Sprintf("selected %s because %s", r.SelectedOrder, r.DecisionReason)
}
