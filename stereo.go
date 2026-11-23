package stereocull

import "math"

// StereoConfig fixes the per-eye frustum geometry in head-local space.
type StereoConfig struct {
	IPD         float64
	Near        float64
	Far         float64
	VerticalFOV float64 // full vertical field of view, radians
	Aspect      float64 // width / height
}

// HalfExtents returns the near-plane half width/height.
func (c StereoConfig) HalfExtents() (halfW, halfH float64) {
	halfH = math.Tan(c.VerticalFOV/2) * c.Near
	halfW = halfH * c.Aspect
	return halfW, halfH
}

func (c StereoConfig) eye(side int) CameraSpec {
	halfW, halfH := c.HalfExtents()
	return CameraSpec{
		EyeX:  float64(side) * c.IPD / 2,
		Near:  c.Near,
		Far:   c.Far,
		HalfW: halfW,
		HalfH: halfH,
	}
}

// LeftEye / RightEye return head-local camera specs (left eye is +X).
func (c StereoConfig) LeftEye() CameraSpec  { return c.eye(+1) }
func (c StereoConfig) RightEye() CameraSpec { return c.eye(-1) }

// HeadPose positions and orients the head in world space.
type HeadPose struct {
	Position Vec3
	Rotation Quat
}

// EyeResult holds one eye's cull outcome and work counters.
type EyeResult struct {
	Accepted IDSet
	Stats    TraversalStats
}

// PipelineReport compares the two culling strategies for one pose.
type PipelineReport struct {
	Left           EyeResult
	Right          EyeResult
	UnionScreen    EyeResult // candidates that survived the binocular union screen
	Walk           StereoStats
	LeftOnly       IDSet
	RightOnly      IDSet
	BothEyes       IDSet
	FinalUnion     IDSet // accepted by at least one eye
	FalsePositives IDSet // union-screen accepted but rejected by both eyes
}

// ExactPerEye runs the reference pipeline: independent precise frustum
// traversal for each eye.
func ExactPerEye(b *BVH, cfg StereoConfig, pose HeadPose) PipelineReport {
	leftFr := BuildFrustum(cfg.LeftEye()).Transform(pose.Rotation, pose.Position)
	rightFr := BuildFrustum(cfg.RightEye()).Transform(pose.Rotation, pose.Position)

	leftAcc, leftStats := b.Cull(leftFr)
	rightAcc, rightStats := b.Cull(rightFr)

	rep := PipelineReport{
		Left:        EyeResult{Accepted: leftAcc, Stats: leftStats},
		Right:       EyeResult{Accepted: rightAcc, Stats: rightStats},
		UnionScreen: EyeResult{Accepted: NewIDSet(len(b.Items))},
	}
	rep.finalizeSets()
	return rep
}

// UnionScreenFast runs the production pipeline: one cheap union-frustom
// traversal rejects everything invisible to both eyes, then each surviving
// candidate is tested precisely against both eye frusta.
func UnionScreenFast(b *BVH, cfg StereoConfig, pose HeadPose) PipelineReport {
	unionFr := BuildUnionFrustum(cfg.LeftEye(), cfg.RightEye()).
		Transform(pose.Rotation, pose.Position)
	leftFr := BuildFrustum(cfg.LeftEye()).Transform(pose.Rotation, pose.Position)
	rightFr := BuildFrustum(cfg.RightEye()).Transform(pose.Rotation, pose.Position)

	leftAcc, rightAcc, screenAcc, walkStats := b.CullStereo(unionFr, leftFr, rightFr)
	rep := PipelineReport{
		UnionScreen: EyeResult{Accepted: screenAcc, Stats: walkStats.Union.Stats},
		Left:        EyeResult{Accepted: leftAcc, Stats: walkStats.Left},
		Right:       EyeResult{Accepted: rightAcc, Stats: walkStats.Right},
		Walk:        walkStats,
	}
	rep.finalizeSets()
	rep.FalsePositives = DiffSet(screenAcc, rep.FinalUnion)
	return rep
}

func (r *PipelineReport) finalizeSets() {
	r.LeftOnly = DiffSet(r.Left.Accepted, r.Right.Accepted)
	r.RightOnly = DiffSet(r.Right.Accepted, r.Left.Accepted)
	r.BothEyes = IntersectSet(r.Left.Accepted, r.Right.Accepted)
	r.FinalUnion = UnionSet(r.Left.Accepted, r.Right.Accepted)
}
