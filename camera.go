package main

import "math"

// StereoRig is a pair of pinhole cameras separated along the local X axis.
type StereoRig struct {
	Position Vec3 // cyclopean (mid-eye) position
	Yaw      float64
	IPD      float64 // interpupillary distance
	Fov      float64 // vertical field of view, radians
}

func (r *StereoRig) Right() Vec3   { return Vec3{math.Cos(r.Yaw), 0, -math.Sin(r.Yaw)} }
func (r *StereoRig) Forward() Vec3 { return Vec3{math.Sin(r.Yaw), 0, math.Cos(r.Yaw)} }
func (r *StereoRig) Up() Vec3      { return Vec3{0, 1, 0} }

// EyePos returns the world position of one eye; eye is 0 (left) or 1 (right).
func (r *StereoRig) EyePos(eye int) Vec3 {
	sign := -0.5
	if eye == 1 {
		sign = 0.5
	}
	return Add(r.Position, Mul(r.Right(), sign*r.IPD))
}

// CameraPath advances the rig along a circular dolly path and records poses.
type CameraPath struct {
	Frames []PathFrame
}

type PathFrame struct {
	Frame  int     `json:"frame"`
	CX     float64 `json:"cx"`
	CY     float64 `json:"cy"`
	CZ     float64 `json:"cz"`
	Yaw    float64 `json:"yaw"`
	LeftX  float64 `json:"lx"`
	LeftY  float64 `json:"ly"`
	LeftZ  float64 `json:"lz"`
	RightX float64 `json:"rx"`
	RightY float64 `json:"ry"`
	RightZ float64 `json:"rz"`
}

// GeneratePath moves the rig on a circle around the scene center, always
// facing inward, and records every stereo pose.
func GeneratePath(numFrames int, ipd float64) (*CameraPath, []StereoRig) {
	path := &CameraPath{}
	rigs := make([]StereoRig, 0, numFrames)
	center := Vec3{0, 0, 6.0}
	radius := 7.0
	for f := 0; f < numFrames; f++ {
		ang := -0.35 + 0.7*float64(f)/float64(numFrames-1)
		pos := Vec3{
			center.X + radius*math.Sin(ang),
			0.2 * math.Sin(2*ang),
			center.Z - radius*math.Cos(ang),
		}
		yaw := math.Atan2(center.X-pos.X, center.Z-pos.Z)
		rig := StereoRig{Position: pos, Yaw: yaw, IPD: ipd, Fov: 60 * math.Pi / 180}
		rigs = append(rigs, rig)
		l, rr := rig.EyePos(0), rig.EyePos(1)
		path.Frames = append(path.Frames, PathFrame{
			Frame: f, CX: pos.X, CY: pos.Y, CZ: pos.Z, Yaw: yaw,
			LeftX: l.X, LeftY: l.Y, LeftZ: l.Z,
			RightX: rr.X, RightY: rr.Y, RightZ: rr.Z,
		})
	}
	return path, rigs
}
