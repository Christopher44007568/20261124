package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"stereocull"
)

type poseCase struct {
	Name string
	Pose stereocull.HeadPose
}

func main() {
	outDir := flag.String("out", "artifacts", "directory for the text report")
	flag.Parse()

	scenes := stereocull.DefaultScenes()
	poses := buildPoses()

	var report strings.Builder
	report.WriteString("stereo frustum culling: union quick-screen vs exact per-eye\n\n")

	failures := 0
	for _, scene := range scenes {
		tree := stereocull.BuildBVH(scene.Instances, 8)
		fmt.Printf("== scene %s instances=%d\n", scene.Kind, len(scene.Instances))
		report.WriteString(fmt.Sprintf("=== scene %s (instances=%d, ipd=%.3f, vfov=%.2frad, near=%.3f) ===\n",
			scene.Kind, len(scene.Instances), scene.Config.IPD, scene.Config.VerticalFOV, scene.Config.Near))

		for _, pc := range poses {
			exact := stereocull.ExactPerEye(tree, scene.Config, pc.Pose)
			fast := stereocull.UnionScreenFast(tree, scene.Config, pc.Pose)

			if err := verifyAgreement(exact, fast); err != nil {
				failures++
				fmt.Printf("  [%s] MISMATCH: %v\n", pc.Name, err)
				report.WriteString(fmt.Sprintf("  [%s] MISMATCH: %v\n", pc.Name, err))
				continue
			}

			exactNs := bench(func() {
				stereocull.ExactPerEye(tree, scene.Config, pc.Pose)
			})
			fastNs := bench(func() {
				stereocull.UnionScreenFast(tree, scene.Config, pc.Pose)
			})
			printPose(pc.Name, fast, exactNs, fastNs)
			writePoseReport(&report, pc.Name, scene, tree, fast, exactNs, fastNs)
		}
		report.WriteString("\n")
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	path := *outDir + "/culling_report.txt"
	if err := os.WriteFile(path, []byte(report.String()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Printf("\nreport written to %s\n", path)
	if failures > 0 {
		fmt.Printf("FAILED: %d mismatches\n", failures)
		os.Exit(1)
	}
	fmt.Println("OK: fast pipeline matches exact per-eye intersection for every pose")
}

func buildPoses() []poseCase {
	return []poseCase{
		{Name: "identity", Pose: stereocull.HeadPose{Rotation: stereocull.QuatEuler(0, 0, 0)}},
		{Name: "yaw-left-35", Pose: stereocull.HeadPose{Rotation: stereocull.QuatEuler(0, math.Pi/180*35, 0)}},
		{Name: "pitch-down-25", Pose: stereocull.HeadPose{Rotation: stereocull.QuatEuler(-math.Pi/180*25, 0, 0)}},
		{Name: "roll-15-yaw-20", Pose: stereocull.HeadPose{Rotation: stereocull.QuatEuler(0, math.Pi/9, math.Pi/12)}},
		{Name: "move-forward-2m", Pose: stereocull.HeadPose{
			Position: stereocull.Vec3{Z: -2},
			Rotation: stereocull.QuatEuler(0, 0, 0),
		}},
		{Name: "move-back-3m-right-1m", Pose: stereocull.HeadPose{
			Position: stereocull.Vec3{X: 1, Z: 3},
			Rotation: stereocull.QuatEuler(0, -math.Pi/7, 0),
		}},
	}
}

func verifyAgreement(exact, fast stereocull.PipelineReport) error {
	if !exact.Left.Accepted.Equal(fast.Left.Accepted) {
		return fmt.Errorf("left eye accepted sets differ")
	}
	if !exact.Right.Accepted.Equal(fast.Right.Accepted) {
		return fmt.Errorf("right eye accepted sets differ")
	}
	if !exact.FinalUnion.Equal(fast.FinalUnion) {
		return fmt.Errorf("final union sets differ")
	}
	if !exact.LeftOnly.Equal(fast.LeftOnly) || !exact.RightOnly.Equal(fast.RightOnly) {
		return fmt.Errorf("single-eye visibility sets differ")
	}
	return nil
}

func bench(fn func()) int64 {
	const runs = 100
	start := time.Now()
	for i := 0; i < runs; i++ {
		fn()
	}
	return time.Since(start).Nanoseconds() / runs
}

func printPose(name string, r stereocull.PipelineReport, exactNs, fastNs int64) {
	speedup := float64(exactNs) / float64(maxInt64(fastNs, 1))
	fmt.Printf("  %-20s L(acc/cand)=%4d/%4d R=%4d/%4d union=%4d ghost=%3d Lonly=%3d Ronly=%3d exact=%6dns fast=%6dns speedup=%.2fx\n",
		name,
		r.Left.Stats.Accepted, r.Left.Stats.Candidates,
		r.Right.Stats.Accepted, r.Right.Stats.Candidates,
		r.UnionScreen.Stats.Accepted, r.FalsePositives.Len(),
		r.LeftOnly.Len(), r.RightOnly.Len(), exactNs, fastNs, speedup)
}

func writePoseReport(b *strings.Builder, name string, scene stereocull.Scene, tree *stereocull.BVH,
	r stereocull.PipelineReport, exactNs, fastNs int64) {
	b.WriteString(fmt.Sprintf("  -- pose %s\n", name))
	writeEye(b, "left", r.Left)
	writeEye(b, "right", r.Right)
	b.WriteString(fmt.Sprintf("    union-screen: candidates=%d accepted=%d rejected=%d nodeTests=%d nodeRejected=%d sphereTests=%d\n",
		r.UnionScreen.Stats.Candidates, r.UnionScreen.Stats.Accepted,
		r.UnionScreen.Stats.Rejected, r.UnionScreen.Stats.NodeTests,
		r.UnionScreen.Stats.NodeRejected, r.UnionScreen.Stats.SphereTests))
	b.WriteString(fmt.Sprintf("    sets: finalUnion=%d bothEyes=%d leftOnly=%d rightOnly=%d unionFalsePositives=%d\n",
		r.FinalUnion.Len(), r.BothEyes.Len(), r.LeftOnly.Len(), r.RightOnly.Len(), r.FalsePositives.Len()))
	b.WriteString("    single-eye special instances: " + namedSpecial(tree, stereocull.UnionSet(r.LeftOnly, r.RightOnly)) + "\n")
	b.WriteString("    union false-positive special instances: " + namedSpecial(tree, r.FalsePositives) + "\n")
	b.WriteString(fmt.Sprintf("    timing: exactPerEye=%dns unionFast=%dns speedup=%.2fx\n\n",
		exactNs, fastNs, float64(exactNs)/float64(maxInt64(fastNs, 1))))
}

func writeEye(b *strings.Builder, label string, eye stereocull.EyeResult) {
	s := eye.Stats
	b.WriteString(fmt.Sprintf("    %s eye: candidates=%d accepted=%d rejected=%d\n",
		label, s.Candidates, s.Accepted, s.Rejected))
}

func namedIDs(tree *stereocull.BVH, ids stereocull.IDSet) string {
	if ids.Len() == 0 {
		return "(none)"
	}
	list := ids.IDs(make([]int, 0, ids.Len()))
	sort.Ints(list)
	parts := make([]string, 0, len(list))
	for _, id := range list {
		ins := tree.ByID(id)
		parts = append(parts, fmt.Sprintf("#%d(%s)", id, ins.Name))
	}
	return strings.Join(parts, ", ")
}

// namedSpecial lists only hand-placed corner-case instances (everything that
// is not a lattice point) so the report stays readable at scale.
func namedSpecial(tree *stereocull.BVH, ids stereocull.IDSet) string {
	special := stereocull.NewIDSet(len(tree.Items))
	gridCount := 0
	for _, id := range ids.IDs(nil) {
		if tree.ByID(id).Name == "grid" {
			gridCount++
			continue
		}
		special.Add(id)
	}
	body := namedIDs(tree, special)
	if gridCount > 0 {
		return fmt.Sprintf("%s (+%d grid instances)", body, gridCount)
	}
	return body
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
