package geom

// Pose is a rigid head pose. Translation is in millimetres.
type Pose struct {
	Q Quat
	T Vec3
}

func (p Pose) Transform(v Vec3) Vec3 {
	return p.Q.Rotate(v).Add(p.T)
}

func InterpolatePose(a, b Pose, u float64) Pose {
	return Pose{
		Q: Slerp(a.Q, b.Q, u),
		T: LerpVec3(a.T, b.T, u),
	}
}

func AngularErrorDeg(a, b Pose) float64 {
	return AngleDeg(a.Q, b.Q)
}

func TranslationErrorMM(a, b Pose) float64 {
	return a.T.Sub(b.T).Len()
}
