package report

import (
	"encoding/json"
	"image"
	"os"
	"path/filepath"

	"timewarp/geom"
	"timewarp/metrics"
	"timewarp/scene"
	"timewarp/warp"
)

// Case is one motion hypothesis and the two warped results for it.
type Case struct {
	ID        string
	Title     string
	Motion    string
	Note      string
	From      geom.Pose
	To        geom.Pose
	GT        *scene.Frame
	Rotation  *warp.Result
	Depth     *warp.Result
	RotStats  metrics.Summary
	DepthStat metrics.Summary
}

// Data is everything needed to write the static report.
type Data struct {
	In        geom.Intrinsics
	FOVYDeg   float64
	Source    *scene.Frame
	Cases     []Case
	PoleWorld geom.Vec3
}

type jsonCase struct {
	ID       string          `json:"id"`
	Title    string          `json:"title"`
	Motion   string          `json:"motion"`
	Rotation metrics.Summary `json:"pure_rotation"`
	Depth    metrics.Summary `json:"depth_reprojection"`
}

type jsonReport struct {
	Width       int        `json:"width"`
	Height      int        `json:"height"`
	FOVYDeg     float64    `json:"fov_y_degrees"`
	Cases       []jsonCase `json:"cases"`
	Conventions []string   `json:"conventions"`
}

// Build writes every diagnostic PNG, metrics.json, and report.html.
func Build(dir string, data Data) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := WritePNG(filepath.Join(dir, "source.png"), FrameImage(data.Source)); err != nil {
		return err
	}
	if err := WritePNG(filepath.Join(dir, "source_depth.png"), DepthImage(data.Source)); err != nil {
		return err
	}
	if err := WritePNG(filepath.Join(dir, "source_regions.png"), RegionImage(data.Source)); err != nil {
		return err
	}

	report := jsonReport{
		Width: data.Source.W, Height: data.Source.H, FOVYDeg: data.FOVYDeg,
		Conventions: []string{
			"pure rotation never reads or writes depth",
			"rotation rejects source coordinates outside the stored frame",
			"depth reprojection never samples outside the source frame",
			"missing source depth is left as a rejection hole, not filled",
		},
	}

	for i := range data.Cases {
		c := &data.Cases[i]
		if err := writeCase(dir, data, c); err != nil {
			return err
		}
		report.Cases = append(report.Cases, jsonCase{
			ID: c.ID, Title: c.Title, Motion: c.Motion,
			Rotation: c.RotStats, Depth: c.DepthStat,
		})
	}
	buf, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "metrics.json"), append(buf, '\n'), 0o644); err != nil {
		return err
	}
	html := renderHTML(data)
	return os.WriteFile(filepath.Join(dir, "report.html"), []byte(html), 0o644)
}

func writeCase(dir string, data Data, c *Case) error {
	files := map[string]*image.RGBA{
		c.ID + "_gt.png":            FrameImage(c.GT),
		c.ID + "_rotation.png":      WarpImage(c.Rotation),
		c.ID + "_depth.png":         WarpImage(c.Depth),
		c.ID + "_rotation_mask.png": MaskImage(c.Rotation),
		c.ID + "_depth_mask.png":    MaskImage(c.Depth),
		c.ID + "_rotation_heat.png": HeatImage(c.GT, c.Rotation),
		c.ID + "_depth_heat.png":    HeatImage(c.GT, c.Depth),
		c.ID + "_regions.png":       RegionImage(c.GT),
	}
	for name, img := range files {
		if err := WritePNG(filepath.Join(dir, name), img); err != nil {
			return err
		}
	}

	pu, pv, _, ok := data.In.Project(c.To.WorldToCam(data.PoleWorld))
	poleRect := RectAround(float64(c.GT.W)/2, float64(c.GT.H)/2, 200, 180, 8, c.GT.W, c.GT.H)
	if ok {
		poleRect = RectAround(pu, pv, 200, 180, 8, c.GT.W, c.GT.H)
	}
	specRect := CoveredRegionBounds(c.GT, c.Rotation, c.Depth, scene.RegionSpecular)
	if specRect.Empty() {
		specRect = RectAround(float64(c.GT.W)/2, float64(c.GT.H)/2, 100, 70, 8, c.GT.W, c.GT.H)
	} else {
		specRect = ExpandRect(specRect, 18, c.GT.W, c.GT.H)
	}
	scale := focusScale(poleRect, 3, 6)
	strip := FocusStrip(
		FrameImage(c.GT), WarpImage(c.Rotation), WarpImage(c.Depth),
		poleRect, scale)
	if err := WritePNG(filepath.Join(dir, c.ID+"_pole_focus.png"), strip); err != nil {
		return err
	}
	scale = focusScale(specRect, 3, 8)
	strip = FocusStrip(
		FrameImage(c.GT), WarpImage(c.Rotation), WarpImage(c.Depth),
		specRect, scale)
	if err := WritePNG(filepath.Join(dir, c.ID+"_specular_focus.png"), strip); err != nil {
		return err
	}
	return nil
}

func focusScale(r image.Rectangle, min, max int) int {
	if r.Dy() <= 0 {
		return min
	}
	scale := 150 / r.Dy()
	if scale < min {
		scale = min
	}
	if scale > max {
		scale = max
	}
	return scale
}
