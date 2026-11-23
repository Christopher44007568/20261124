package geo

import (
	"math"
	"testing"
)

func TestProjectUnprojectRoundTrip(t *testing.T) {
	k := Intrinsics{W: 320, H: 240, Fx: 360, Fy: 360, Cx: 160, Cy: 120}
	pose := Pose{C: Vec3{X: 0.1, Y: -0.2, Z: 0.3}, Rot: RotYX(0.05, -0.02)}
	world := Vec3{X: 1.3, Y: 0.7, Z: 5.2}
	cam := pose.ToCamera(world)
	u, v, z, ok := k.Project(cam)
	if !ok {
		t.Fatalf("project rejected in-frame point")
	}
	back := k.Unproject(u, v, z, pose)
	if d := Norm(Sub(back, world)); d > 1e-9 {
		t.Fatalf("round trip mismatch: %v vs %v (d=%g)", back, world, d)
	}
}

func TestStereoRigBaseline(t *testing.T) {
	rig := Rig{Baseline: 0.12}
	rot := RotYX(0, 0)
	l := rig.EyeCenter(Vec3{}, rot, Left)
	r := rig.EyeCenter(Vec3{}, rot, Right)
	if got := math.Abs(l.X - r.X); math.Abs(got-rig.Baseline) > 1e-12 {
		t.Fatalf("baseline = %v, want %v", got, rig.Baseline)
	}
}

func TestPathContinuity(t *testing.T) {
	p := Path{Duration: 2.0, PosAmplitude: Vec3{X: 1, Y: 0.2, Z: 0.3}, YawAmp: 0.1, PitchAmp: 0.05}
	const n = 100
	maxJump := 0.0
	prev := p.Sample(0)
	for i := 1; i <= n; i++ {
		tt := p.Duration * float64(i) / n
		cur := p.Sample(tt)
		if jump := Norm(Sub(cur.C, prev.C)); jump > maxJump {
			maxJump = jump
		}
		prev = cur
	}
	// A dense sampling of the analytic path must move in small, bounded steps.
	if maxJump > 0.08 {
		t.Fatalf("path not smooth, max jump %v", maxJump)
	}
}
