package main

import (
	"encoding/json"
	"os"
)

type LODReport struct {
	Level       int     `json:"level"`
	Name        string  `json:"name"`
	Vertices    int     `json:"vertices"`
	Triangles   int     `json:"triangles"`
	TargetRatio float64 `json:"targetRatio"`
}

type TransitionReport struct {
	From    int       `json:"from"`
	To      int       `json:"to"`
	Metrics MetricSet `json:"metrics"`
}

type ScenarioReport struct {
	Name        string             `json:"name"`
	Camera      Camera             `json:"camera"`
	Transitions []TransitionReport `json:"transitions"`
	Pass        bool               `json:"pass"`
}

type Report struct {
	Model      string           `json:"model"`
	Resolution [2]int           `json:"resolution"`
	PassRule   string           `json:"passRule"`
	LODs       []LODReport      `json:"lods"`
	Scenarios  []ScenarioReport `json:"scenarios"`
	AllPassed  bool             `json:"allPassed"`
}

func main() {
	source := makeAuditMesh()
	lods := buildLODs(source)
	cameras := auditCameras()

	report := Report{
		Model:      source.Name,
		Resolution: [2]int{256, 256},
		PassRule:   "any missing reference coverage pixel is a one-pixel silhouette failure",
		LODs:       make([]LODReport, len(lods)),
		AllPassed:  true,
	}

	for index, mesh := range lods {
		ratio := float64(len(mesh.Triangles)) / float64(len(source.Triangles))
		report.LODs[index] = LODReport{
			Level:       index,
			Name:        mesh.Name,
			Vertices:    len(mesh.Vertices),
			Triangles:   len(mesh.Triangles),
			TargetRatio: ratio,
		}
	}

	for _, camera := range cameras {
		frames := make([]RenderBuffer, len(lods))
		for index, mesh := range lods {
			frames[index] = renderMesh(mesh, camera)
		}

		scenario := ScenarioReport{Name: camera.Name, Camera: camera, Pass: true}
		for from := 0; from+1 < len(lods); from++ {
			to := from + 1
			metrics := calculateMetrics(frames[from], frames[to], lods[from], lods[to], camera)
			scenario.Transitions = append(scenario.Transitions, TransitionReport{From: from, To: to, Metrics: metrics})
			reverse := calculateMetrics(frames[to], frames[from], lods[to], lods[from], camera)
			scenario.Transitions = append(scenario.Transitions, TransitionReport{From: to, To: from, Metrics: reverse})
			if !metrics.Pass || !reverse.Pass {
				scenario.Pass = false
			}
		}

		if !scenario.Pass {
			report.AllPassed = false
		}
		report.Scenarios = append(report.Scenarios, scenario)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		panic(err)
	}
}

func buildLODs(source *Mesh) []*Mesh {
	baseTarget := len(source.Triangles)
	return []*Mesh{
		source,
		simplifyMesh(source, (baseTarget*62)/100, "lod-1"),
		simplifyMesh(source, (baseTarget*34)/100, "lod-2"),
	}
}

func auditCameras() []Camera {
	return []Camera{
		{Name: "front", Position: Vec3{0, 0, 3}, Target: Vec3{}, Up: Vec3{0, 1, 0}, Zoom: 1.08, Width: 256, Height: 256},
		{Name: "grazing", Position: Vec3{2.85, 0.28, 0.48}, Target: Vec3{}, Up: Vec3{0, 1, 0}, Zoom: 1.08, Width: 256, Height: 256},
		{Name: "fast-zoom", Position: Vec3{0, 0, 3}, Target: Vec3{}, Up: Vec3{0, 1, 0}, Zoom: 2.55, Width: 256, Height: 256},
		{Name: "occlusion-reappear", Position: Vec3{0, 0, 3}, Target: Vec3{}, Up: Vec3{0, 1, 0}, Zoom: 1.08, Width: 256, Height: 256, Occluder: true},
	}
}
