package shadow

import "math"

// EyeMask identifies the eyes for which a caster is relevant.
type EyeMask uint8

const (
	LeftEye  EyeMask = 1 << 0
	RightEye EyeMask = 1 << 1
	BothEyes         = LeftEye | RightEye
)

func (m EyeMask) Has(eye EyeMask) bool { return m&eye != 0 }

// Eye is a rendered camera pose.
type Eye struct {
	Name  string
	Side  float64
	Pos   Vec3
	Yaw   float64
	Pitch float64
	HFOV  float64
	VFOV  float64
}

// Scene owns the caster geometry and the stereo rig parameters.
type Scene struct {
	Casters       []Caster
	Bounds        Bounds
	EyeSeparation float64
	EyeHeight     float64
	BasePitch     float64
	HFOV, VFOV    float64
}

// DefaultScene includes a blocking wall, one rod visible to each eye alone,
// two cross-eye instances, and the wall itself as a shared caster.
func DefaultScene() Scene {
	return Scene{
		Casters: []Caster{
			{Name: "occluder-wall", Kind: BoxCaster, Center: Vec2{0, 8},
				Half: Vec2{0.9, 0.22}, Height: 2.8, BlocksSight: true},
			{Name: "left-eye-rod", Kind: RodCaster, Center: Vec2{-1.4, 12},
				Radius: 0.055, Height: 2.2},
			{Name: "right-eye-rod", Kind: RodCaster, Center: Vec2{1.4, 12},
				Radius: 0.055, Height: 2.2},
			{Name: "cross-eye-near", Kind: BoxCaster, Center: Vec2{2.3, 5.2},
				Half: Vec2{0.5, 0.35}, Height: 1.2},
			{Name: "cross-eye-far", Kind: BoxCaster, Center: Vec2{-2.3, 5.2},
				Half: Vec2{0.5, 0.35}, Height: 1.2},
		},
		Bounds:        Bounds{MinX: -7, MaxX: 7, MinZ: -1, MaxZ: 21},
		EyeSeparation: 0.064,
		EyeHeight:     1.6,
		BasePitch:     -0.12,
		HFOV:          math.Pi * 0.42,
		VFOV:          math.Pi * 0.30,
	}
}

// Pose builds the left (side=-1) or right (side=+1) eye after a head yaw.
// Moving the eyes around the head center makes rapid turns affect parallax,
// not just the camera direction.
func (s Scene) Pose(side, yaw, pitch float64) Eye {
	half := s.EyeSeparation * 0.5
	pos := Vec3{
		X: side * half * math.Cos(yaw),
		Y: s.EyeHeight,
		Z: -side * half * math.Sin(yaw),
	}
	name := "left"
	if side > 0 {
		name = "right"
	}
	return Eye{Name: name, Side: side, Pos: pos, Yaw: yaw, Pitch: pitch,
		HFOV: s.HFOV, VFOV: s.VFOV}
}

func (e Eye) forward() Vec3 {
	cp := math.Cos(e.Pitch)
	return Vec3{X: math.Sin(e.Yaw) * cp, Y: math.Sin(e.Pitch), Z: math.Cos(e.Yaw) * cp}
}

func (e Eye) sees(c Caster, blockers []Caster) bool {
	targetY := math.Min(c.Height*0.5, e.Pos.Y*0.85)
	target := Vec3{X: c.Center.X, Y: targetY, Z: c.Center.Z}
	to := target.Sub(e.Pos)
	horiz := math.Hypot(to.X, to.Z)
	if horiz < eps || horiz > 30 {
		return false
	}
	forwardH := Vec3{X: math.Sin(e.Yaw), Z: math.Cos(e.Yaw)}
	dirH := Vec3{X: to.X / horiz, Z: to.Z / horiz}
	if forwardH.Dot(dirH) < math.Cos(e.HFOV*0.5) {
		return false
	}
	pitch := math.Atan2(to.Y, horiz)
	if math.Abs(pitch-e.Pitch) > e.VFOV*0.5 {
		return false
	}
	for _, o := range blockers {
		if o.Name != c.Name && segmentHitsCaster(e.Pos, target, o) {
			return false
		}
	}
	return true
}

// CasterMasks computes per-eye relevance for the current head pose.
// A caster is relevant only if its center passes that eye's frustum and
// line-of-sight test. Bits are retained independently for both eyes.
func (s Scene) CasterMasks(yaw, pitch float64) []EyeMask {
	eyes := [2]Eye{s.Pose(-1, yaw, pitch), s.Pose(1, yaw, pitch)}
	blockers := make([]Caster, 0, len(s.Casters))
	for _, c := range s.Casters {
		if c.BlocksSight {
			blockers = append(blockers, c)
		}
	}
	masks := make([]EyeMask, len(s.Casters))
	for i, c := range s.Casters {
		if eyes[0].sees(c, blockers) {
			masks[i] |= LeftEye
		}
		if eyes[1].sees(c, blockers) {
			masks[i] |= RightEye
		}
	}
	return masks
}

func countMask(masks []EyeMask, eye EyeMask) int {
	n := 0
	for _, m := range masks {
		if m.Has(eye) {
			n++
		}
	}
	return n
}

func countUnion(masks []EyeMask) int {
	n := 0
	for _, m := range masks {
		if m != 0 {
			n++
		}
	}
	return n
}

// LightDirection returns a unit direction from the ground toward the light.
func LightDirection(azimuth, elevation float64) Vec3 {
	ce := math.Cos(elevation)
	return Vec3{
		X: ce * math.Sin(azimuth),
		Y: math.Sin(elevation),
		Z: ce * math.Cos(azimuth),
	}
}
