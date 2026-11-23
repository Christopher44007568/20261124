// Command stereohzb demonstrates stereo HZB occlusion culling: far layers
// are shared between eyes, near layers are rebuilt per eye, and conservative
// replays keep the culler exact. It runs a control pass without recheck to
// show traceable false-cull events, then the protected pass.
package main

import (
	"fmt"
	"math"

	"stereohzb/hzb"
)

func cam(id string, x, yawDeg float64) *hzb.Camera {
	rad := yawDeg * math.Pi / 180
	return &hzb.Camera{
		ID:       id,
		Position: hzb.Vec3{X: x, Y: 0, Z: 0},
		Forward:  hzb.Vec3{X: math.Sin(rad), Y: 0, Z: math.Cos(rad)},
		Up:       hzb.Vec3{X: 0, Y: 1, Z: 0},
		FovYDeg:  60, Near: 0.1, Far: 100, Width: 64, Height: 64,
	}
}

func box(x0, y0, z0, x1, y1, z1 float64) hzb.AABB {
	return hzb.AABB{Min: hzb.Vec3{X: x0, Y: y0, Z: z0}, Max: hzb.Vec3{X: x1, Y: y1, Z: z1}}
}

func buildScene() *hzb.Scene {
	s := &hzb.Scene{}
	s.Occluders = append(s.Occluders, hzb.BoxTriangles(hzb.Vec3{X: -1.0, Y: -1, Z: 4.9}, hzb.Vec3{X: 0.0, Y: 1, Z: 5.1})...)
	s.Occluders = append(s.Occluders, hzb.BoxTriangles(hzb.Vec3{X: 0.77, Y: -2, Z: 5.0}, hzb.Vec3{X: 0.83, Y: 2, Z: 5.06})...)
	s.Targets = []hzb.Object{
		{ID: "behind-wall", Box: box(-0.7, -0.3, 6.1, -0.1, 0.3, 6.5)},
		{ID: "behind-rod", Box: box(0.2, -0.6, 6.05, 1.4, 0.6, 6.35)},
		{ID: "revealed-later", Box: box(0.1, -0.3, 7.1, 0.7, 0.3, 7.5)},
		{ID: "free-space", Box: box(2.5, 0.0, 6.0, 3.1, 0.6, 6.4)},
	}
	return s
}

func runFrame(frame int, camL, camR *hzb.Camera, sc *hzb.Scene, pol hzb.SharePolicy) *hzb.StereoHZB {
	dbL := hzb.RenderDepth(camL, sc.Occluders)
	dbR := hzb.RenderDepth(camR, sc.Occluders)
	s := hzb.BuildStereo(camL, camR, dbL, dbR, frame, pol)
	s.ValidateEye(camL, 0, dbL, sc.Targets)
	s.ValidateEye(camR, 1, dbR, sc.Targets)
	return s
}

func report(tag string, s *hzb.StereoHZB) {
	st := s.Stats
	fmt.Printf("%-22s q=%d culled=%d shared-q=%.0f%% reuse=%.1f%% replay=%d(save=%d px=%d) false=%d(px=%d) build=%s query=%s\n",
		tag, st.Queries, st.Occluded, 100*st.SharedQueryRatio(), 100*st.ReuseRatio(),
		st.Replays, st.ReplaySaves, st.ReplayPixels,
		st.FalseCulls, st.FalseCullPixels, st.BuildTime, st.QueryTime)
	for _, ev := range st.FalseCullEvents {
		fmt.Printf("    ! %s\n", ev)
	}
}

func main() {
	sc := buildScene()
	protected := hzb.SharePolicy{SharedFromLevel: 2, RecheckShared: true, RecheckCoarse: true}
	control := hzb.SharePolicy{SharedFromLevel: 2}

	// Frame 1: baseline. Frame 2: small rotation + translation (parallax).
	// Frame 3: larger translation that reveals the hidden target.
	frames := []struct{ x, yaw float64 }{{0, 0}, {0.15, 2}, {0.6, 2}}
	for i, f := range frames {
		camL := cam("left", f.x-0.05, f.yaw)
		camR := cam("right", f.x+0.05, f.yaw)
		fmt.Printf("frame %d (x=%.2f yaw=%.1f)\n", i+1, f.x, f.yaw)
		report("  no-recheck (control)", runFrame(i+1, camL, camR, sc, control))
		report("  recheck (protected)", runFrame(i+1, camL, camR, sc, protected))
	}

	// Different near planes: sharing must be refused.
	camL := cam("left", -0.05, 0)
	camR := cam("right", 0.05, 0)
	camR.Near = 0.9
	fmt.Println("near-plane mismatch (left=0.1 right=0.9)")
	report("  fallback per-eye", runFrame(4, camL, camR, sc, protected))
}
