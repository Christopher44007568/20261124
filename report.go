package gazedenoise

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ExperimentOptions configures one reproducible comparison run.
type ExperimentOptions struct {
	OutDir string
	Width  int
	Height int
	Noise  float64
	Seed   int64
	Scenes []SceneKind
	Params Params
}

// DefaultExperimentOptions returns the preview settings used by the CLI.
func DefaultExperimentOptions() ExperimentOptions {
	return ExperimentOptions{
		OutDir: "artifacts/report",
		Width:  192,
		Height: 144,
		Noise:  0.035,
		Seed:   20261124,
		Scenes: SceneKinds(),
		Params: DefaultParams(),
	}
}

// ModeReport is one filtered result in a case.
type ModeReport struct {
	Mode            string  `json:"mode"`
	Metrics         Metrics `json:"metrics"`
	Image           string  `json:"image"`
	DifferenceImage string  `json:"difference_image"`
	ProvenanceJSON  string  `json:"provenance_json"`
}

// CaseReport groups all modes for one synthetic stress scene.
type CaseReport struct {
	Kind            string         `json:"kind"`
	InputHoles      int            `json:"input_holes"`
	FeaturePixels   map[string]int `json:"feature_pixels"`
	TruthImage      string         `json:"truth_image"`
	InputImage      string         `json:"input_image"`
	MaskImage       string         `json:"mask_image"`
	NormalImage     string         `json:"normal_image"`
	DepthImage      string         `json:"depth_image"`
	LumaImage       string         `json:"luma_image"`
	ProvenanceImage string         `json:"provenance_image"`
	Modes           []ModeReport   `json:"modes"`
}

// Report is the JSON/Markdown experiment manifest.
type Report struct {
	Format string       `json:"format"`
	Width  int          `json:"width"`
	Height int          `json:"height"`
	Noise  float64      `json:"noise"`
	Seed   int64        `json:"seed"`
	Params Params       `json:"params"`
	Cases  []CaseReport `json:"cases"`
}

// ProvenanceDocument is the durable per-mode hole-source record.
type ProvenanceDocument struct {
	Format        string           `json:"format"`
	Case          string           `json:"case"`
	Mode          string           `json:"mode"`
	FilledHoles   int              `json:"filled_holes"`
	UnfilledHoles int              `json:"unfilled_holes"`
	Holes         []HoleProvenance `json:"holes"`
}

// RunExperiment renders all cases, writes PNG evidence, and returns a report.
func RunExperiment(opts ExperimentOptions) (*Report, error) {
	if opts.Width < 24 || opts.Height < 24 {
		return nil, fmt.Errorf("invalid experiment size %dx%d", opts.Width, opts.Height)
	}
	if opts.OutDir == "" {
		return nil, fmt.Errorf("output directory is required")
	}
	if err := opts.Params.Validate(); err != nil {
		return nil, err
	}
	if len(opts.Scenes) == 0 {
		opts.Scenes = SceneKinds()
	}
	outRoot, err := filepath.Abs(opts.OutDir)
	if err != nil {
		return nil, err
	}
	opts.OutDir = outRoot
	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return nil, err
	}

	report := &Report{
		Format: "gazedenoise/v1",
		Width:  opts.Width,
		Height: opts.Height,
		Noise:  opts.Noise,
		Seed:   opts.Seed,
		Params: opts.Params,
	}
	modes := []Mode{ModeRaw, ModePartitioned, ModeFull}
	for _, kind := range opts.Scenes {
		scene, err := GenerateScene(kind, opts.Width, opts.Height, opts.Noise, opts.Seed)
		if err != nil {
			return nil, err
		}
		caseDir := filepath.Join(opts.OutDir, string(kind))
		truthPath := filepath.Join(caseDir, "truth.png")
		inputPath := filepath.Join(caseDir, "input.png")
		maskPath := filepath.Join(caseDir, "mask.png")
		normalPath := filepath.Join(caseDir, "normal.png")
		depthPath := filepath.Join(caseDir, "depth.png")
		lumaPath := filepath.Join(caseDir, "luma.png")
		if err := WriteFramePNG(truthPath, scene.Truth, RenderOptions{MaskOutline: true}); err != nil {
			return nil, err
		}
		if err := WriteFramePNG(inputPath, scene.Noisy, RenderOptions{MaskOutline: true}); err != nil {
			return nil, err
		}
		if err := WriteFramePNG(maskPath, maskFrame(scene), RenderOptions{MaskOutline: true}); err != nil {
			return nil, err
		}
		if err := WriteNormalPNG(normalPath, scene.Truth, RenderOptions{MaskOutline: true}); err != nil {
			return nil, err
		}
		if err := WriteScalarPNG(depthPath, scene.Truth, scene.Truth.Depth, 0, 0,
			RenderOptions{MaskOutline: true}); err != nil {
			return nil, err
		}
		if err := WriteScalarPNG(lumaPath, scene.Truth, scene.Truth.Luma, 0, 0,
			RenderOptions{MaskOutline: true}); err != nil {
			return nil, err
		}

		caseReport := CaseReport{
			Kind:            string(kind),
			TruthImage:      relative(opts.OutDir, truthPath),
			InputImage:      relative(opts.OutDir, inputPath),
			MaskImage:       relative(opts.OutDir, maskPath),
			NormalImage:     relative(opts.OutDir, normalPath),
			DepthImage:      relative(opts.OutDir, depthPath),
			LumaImage:       relative(opts.OutDir, lumaPath),
			ProvenanceImage: filepath.ToSlash(filepath.Join(string(kind), "provenance-partitioned.png")),
			FeaturePixels:   make(map[string]int),
		}
		for _, valid := range scene.Noisy.Valid {
			if !valid {
				caseReport.InputHoles++
			}
		}
		for _, name := range scene.FeatureNames() {
			for _, selected := range scene.Feature(name) {
				if selected {
					caseReport.FeaturePixels[name]++
				}
			}
		}

		for _, mode := range modes {
			result, err := Denoise(scene.Noisy, mode, opts.Params)
			if err != nil {
				return nil, fmt.Errorf("%s/%s: %w", kind, mode, err)
			}
			metrics, err := Evaluate(scene, result)
			if err != nil {
				return nil, fmt.Errorf("%s/%s metrics: %w", kind, mode, err)
			}
			imageRel := filepath.ToSlash(filepath.Join(string(kind), mode.String()+".png"))
			diffRel := filepath.ToSlash(filepath.Join(string(kind), mode.String()+"-diff.png"))
			provenanceRel := filepath.ToSlash(filepath.Join(string(kind), "provenance-"+mode.String()+".json"))
			if err := WriteFramePNG(filepath.Join(opts.OutDir, filepath.FromSlash(imageRel)), result.Output,
				RenderOptions{MaskOutline: true}); err != nil {
				return nil, err
			}
			if err := WriteDifferencePNG(filepath.Join(opts.OutDir, filepath.FromSlash(diffRel)),
				scene.Truth, result.Output, 12); err != nil {
				return nil, err
			}
			if mode == ModePartitioned {
				if err := WriteProvenancePNG(filepath.Join(opts.OutDir, string(kind), "provenance-partitioned.png"),
					scene, result); err != nil {
					return nil, err
				}
			}
			if err := writeProvenanceJSON(filepath.Join(opts.OutDir, filepath.FromSlash(provenanceRel)),
				string(kind), mode.String(), result); err != nil {
				return nil, err
			}
			caseReport.Modes = append(caseReport.Modes, ModeReport{
				Mode:            mode.String(),
				Metrics:         metrics,
				Image:           imageRel,
				DifferenceImage: diffRel,
				ProvenanceJSON:  provenanceRel,
			})
		}
		report.Cases = append(report.Cases, caseReport)
	}

	if err := writeReportJSON(filepath.Join(opts.OutDir, "report.json"), report); err != nil {
		return nil, err
	}
	if err := writeReportMarkdown(filepath.Join(opts.OutDir, "report.md"), report); err != nil {
		return nil, err
	}
	return report, nil
}

func maskFrame(scene *Scene) *Frame {
	out, _ := NewFrame(scene.Width, scene.Height)
	for i := range out.Color {
		out.Mask[i] = scene.Truth.Mask[i]
		out.Valid[i] = true
		if out.Mask[i] {
			out.Color[i] = Vec3{X: 0.10, Y: 0.36, Z: 0.42}
		} else {
			out.Color[i] = Vec3{X: 0.04, Y: 0.05, Z: 0.07}
		}
	}
	out.RecomputeLuma()
	return out
}

func relative(base, target string) string {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return filepath.ToSlash(target)
	}
	return filepath.ToSlash(rel)
}

func writeReportJSON(path string, report *Report) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func writeProvenanceJSON(path, caseName, mode string, result *Result) error {
	holes := result.Provenance
	if holes == nil {
		holes = []HoleProvenance{}
	}
	doc := ProvenanceDocument{
		Format:        "gazedenoise/provenance-v1",
		Case:          caseName,
		Mode:          mode,
		FilledHoles:   len(holes),
		UnfilledHoles: result.UnfilledHoles,
		Holes:         holes,
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func writeReportMarkdown(path string, report *Report) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("# 注视边界降噪审计\n\n")
	fmt.Fprintf(&b, "- 图像：%dx%d，颜色噪声 σ=%.3f，随机种子=%d\n",
		report.Width, report.Height, report.Noise, report.Seed)
	fmt.Fprintf(&b, "- 滤波：半径=%d，空间 σ=%.2f，法线 σ=%.2f，深度 σ=%.2f，亮度 σ=%.3f，补洞半径=%d\n",
		report.Params.Radius, report.Params.SigmaSpatial, report.Params.SigmaNormal,
		report.Params.SigmaDepth, report.Params.SigmaLuma, report.Params.HoleRadius)
	b.WriteString("- 分区策略：中心像素不平滑；外围所有滤波 tap 和补洞供体必须与目标同属一个注视分区。\n")
	b.WriteString("- 全图模式：故意不检查注视分区，仅作为越界光晕对照基线。\n\n")

	badPolicy := make([]string, 0)
	badProvenance := make([]string, 0)
	for _, c := range report.Cases {
		for _, mode := range c.Modes {
			if mode.Mode == ModePartitioned.String() && mode.Metrics.CrossPartitionTaps != 0 {
				badPolicy = append(badPolicy, c.Kind)
			}
			if !mode.Metrics.ProvenanceValid {
				badProvenance = append(badProvenance, c.Kind+"/"+mode.Mode)
			}
		}
	}
	if len(badPolicy) == 0 {
		b.WriteString("**分区审计：通过。所有分区结果跨区 tap 为 0，中心样本未进入外围权重。**\n\n")
	} else {
		fmt.Fprintf(&b, "**分区审计：失败。越界场景：%s。**\n\n", strings.Join(badPolicy, ", "))
	}
	if len(badProvenance) == 0 {
		b.WriteString("**补洞来源审计：通过。每个目标都有非空供体、权重和为 1，且记录可重建输出。**\n\n")
	} else {
		fmt.Fprintf(&b, "**补洞来源审计：失败。%s。**\n\n", strings.Join(badProvenance, ", "))
	}

	b.WriteString("## 重建与边界指标\n\n")
	b.WriteString("| 场景 | 模式 | 中心 MAE | 外围 MAE | 边界 RMSE | 光晕偏置 | 边缘能量比 | 跨区 taps | 跨区权重 | 补洞 | 来源校验 |\n")
	b.WriteString("|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|\n")
	for _, c := range report.Cases {
		for _, mode := range c.Modes {
			m := mode.Metrics
			fmt.Fprintf(&b, "| %s | %s | %.5f | %.5f | %.5f | %.5f | %.3f | %d | %.5f | %d | %s |\n",
				c.Kind, mode.Mode, m.CentreMAE, m.PeripheryMAE, m.BoundaryRMSE,
				m.HaloBias, m.EdgeEnergyRatio, m.CrossPartitionTaps, m.CrossPartitionWeight,
				m.FilledHoles, boolLabel(m.ProvenanceValid))
		}
	}

	b.WriteString("\n## 压力区域\n\n")
	b.WriteString("单元格为 `MAE / 边缘能量比`。细杆、半透明边和曝光跳变均跨过注视边界。\n\n")
	b.WriteString("| 场景 | 特性 | 像素 | raw | partitioned | full |\n")
	b.WriteString("|---|---:|---:|---:|---:|---:|\n")
	for _, c := range report.Cases {
		for _, feature := range sortedCountKeys(c.FeaturePixels) {
			raw, _ := metricsForMode(c, ModeRaw.String())
			part, _ := metricsForMode(c, ModePartitioned.String())
			full, _ := metricsForMode(c, ModeFull.String())
			fmt.Fprintf(&b, "| %s | %s | %d | %s | %s | %s |\n",
				c.Kind, feature, c.FeaturePixels[feature],
				stressCell(raw, feature), stressCell(part, feature), stressCell(full, feature))
		}
	}

	b.WriteString("\n## 证据文件\n\n")
	for _, c := range report.Cases {
		fmt.Fprintf(&b, "- %s：truth=`%s`，normal=`%s`，depth=`%s`，luma=`%s`，input=`%s`，provenance=`%s`，diff=`%s`\n",
			c.Kind, c.TruthImage, c.NormalImage, c.DepthImage, c.LumaImage,
			c.InputImage, c.ProvenanceImage, c.Modes[len(c.Modes)-1].DifferenceImage)
		for _, mode := range c.Modes {
			fmt.Fprintf(&b, "  - %s：`%s`，sources=`%s`\n",
				mode.Mode, mode.Image, mode.ProvenanceJSON)
		}
	}
	b.WriteString("\n差异图的红色表示输出比真值亮，蓝色表示比真值暗；青色为注视边界。\n")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func metricsForMode(c CaseReport, mode string) (Metrics, bool) {
	for _, candidate := range c.Modes {
		if candidate.Mode == mode {
			return candidate.Metrics, true
		}
	}
	return Metrics{}, false
}

func stressCell(m Metrics, feature string) string {
	mae, ok := m.StressMAE[feature]
	if !ok {
		return "-"
	}
	return fmt.Sprintf("%.5f / %.2f", mae, m.StressEdgeEnergyRatio[feature])
}

func boolLabel(ok bool) string {
	if ok {
		return "通过"
	}
	return "失败"
}

func sortedCountKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
