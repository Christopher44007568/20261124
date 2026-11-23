package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
)

const (
	numFrames   = 24
	imgWidth    = 320
	imgHeight   = 180
	ipd         = 0.12
	artifactDir = "artifacts"
)

func writeJSONL(path string, rows []any) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, r := range rows {
		if err := enc.Encode(r); err != nil {
			return err
		}
	}
	return nil
}

func mustPNG(path string, img image.Image) {
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		panic(err)
	}
}

func main() {
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		panic(err)
	}
	scene := buildScene()
	path, rigs := GeneratePath(numFrames, ipd)

	// Persist the stereo camera path.
	pathRows := make([]any, len(path.Frames))
	for i, pf := range path.Frames {
		pathRows[i] = pf
	}
	if err := writeJSONL(filepath.Join(artifactDir, "camera_path.jsonl"), pathRows); err != nil {
		panic(err)
	}

	// Run the two-layer query for every frame and log metrics.
	logRows := make([]any, 0, numFrames)
	var totalShared, totalComplement, totalFallback, totalFalse int
	sharedFailures := 0
	results := make([]*QueryResult, numFrames)
	for f := 0; f < numFrames; f++ {
		res := RunQuery(scene, &rigs[f], f)
		Validate(scene, &rigs[f], res)
		results[f] = res
		m := res.Metrics
		totalShared += m.SharedTraversal
		totalComplement += m.ComplementTraversal
		totalFallback += m.ConservativeFallback
		totalFalse += m.FalseCulled
		if m.SharedFailure {
			sharedFailures++
		}
		logRows = append(logRows, m)
	}
	if err := writeJSONL(filepath.Join(artifactDir, "query_log.jsonl"), logRows); err != nil {
		panic(err)
	}

	// Render the frame with the most visible objects from both eyes,
	// plus the stereo difference.
	best, bestVisible := 0, -1
	for f, res := range results {
		v := res.Metrics.VisibleLeft + res.Metrics.VisibleRight
		if v > bestVisible {
			best, bestVisible = f, v
		}
	}
	rig := rigs[best]
	left := RenderEye(scene, &rig, 0, imgWidth, imgHeight)
	right := RenderEye(scene, &rig, 1, imgWidth, imgHeight)
	diff := FrameDiff(left, right)
	mustPNG(filepath.Join(artifactDir, "frame_left.png"), left)
	mustPNG(filepath.Join(artifactDir, "frame_right.png"), right)
	mustPNG(filepath.Join(artifactDir, "frame_diff.png"), diff)

	// Summary with the false-cull rate over all culling decisions.
	totalCulled := 0
	for _, res := range results {
		totalCulled += len(res.CulledShared)*2 + len(res.CulledPerEye[0]) + len(res.CulledPerEye[1])
	}
	falseRate := 0.0
	if totalCulled > 0 {
		falseRate = float64(totalFalse) / float64(totalCulled)
	}
	summary := map[string]any{
		"frames":                 numFrames,
		"rendered_frame":         best,
		"objects":                len(scene.Objects),
		"shared_occluders":       len(scene.SharedSet()),
		"complement_occluders":   len(scene.ComplementSet()),
		"shared_traversal":       totalShared,
		"complement_traversal":   totalComplement,
		"conservative_fallback":  totalFallback,
		"total_culled_decisions": totalCulled,
		"false_culled":           totalFalse,
		"false_cull_rate":        falseRate,
		"shared_failure_frames":  sharedFailures,
	}
	out, _ := json.MarshalIndent(summary, "", "  ")
	fmt.Println(string(out))
	if sharedFailures > 0 {
		fmt.Println("RESULT: FAILED - shared occluder set culled an object visible to an eye")
		os.Exit(1)
	}
	fmt.Println("RESULT: OK - shared set conservative for both eyes")
}
