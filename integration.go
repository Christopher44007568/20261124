package posetimeline

// Motion is the accumulated rigid-body movement relative to the first
// accepted baseline pose. Integration only advances across accepted samples
// of an unbroken timeline, so an invalidated timeline contributes nothing
// until Reset and a new baseline arrive.
type Motion struct {
	Translation Vec3 // cumulative translation, meters
	Rotation    Quat // cumulative orientation change
	Steps       int  // number of accepted samples integrated after baseline
	Valid       bool // false until a baseline exists or while broken
}

// IntegratedMotion reports accumulated motion for one eye.
func (v *Validator) IntegratedMotion(eye Eye) Motion {
	if int(eye) >= len(v.eyes) {
		return Motion{}
	}
	state := v.eyes[eye]
	if state.broken || !state.havePose || len(state.accepted) == 0 {
		return Motion{Valid: false}
	}

	baseline := state.accepted[0].sample.Pose
	rotation := quatDifference(state.lastPose.Orientation, baseline.Orientation)
	translation := vecSub(state.lastPose.Position, baseline.Position)
	return Motion{
		Translation: translation,
		Rotation:    rotation,
		Steps:       state.integrated - 1,
		Valid:       true,
	}
}

// LastAcceptedPose returns the most recent accepted pose for an eye and its
// translated timestamp. It returns ok=false on an empty or broken timeline.
func (v *Validator) LastAcceptedPose(eye Eye) (Pose, int64, bool) {
	if int(eye) >= len(v.eyes) {
		return Pose{}, 0, false
	}
	state := v.eyes[eye]
	if state.broken || len(state.accepted) == 0 {
		return Pose{}, 0, false
	}
	return state.lastPose, state.lastTime, true
}

func vecSub(a, b Vec3) Vec3 {
	return Vec3{X: a.X - b.X, Y: a.Y - b.Y, Z: a.Z - b.Z}
}

// quatDifference returns q = target * conj(reference), the rotation that
// takes reference orientation onto target.
func quatDifference(target, reference Quat) Quat {
	inv := Quat{W: reference.W, X: -reference.X, Y: -reference.Y, Z: -reference.Z}
	return quatMul(target, inv)
}

func quatMul(a, b Quat) Quat {
	return Quat{
		W: a.W*b.W - a.X*b.X - a.Y*b.Y - a.Z*b.Z,
		X: a.W*b.X + a.X*b.W + a.Y*b.Z - a.Z*b.Y,
		Y: a.W*b.Y - a.X*b.Z + a.Y*b.W + a.Z*b.X,
		Z: a.W*b.Z + a.X*b.Y - a.Y*b.X + a.Z*b.W,
	}
}
