package reprojection

import "math"

const nearClip = 1e-5

// Camera defines the display raster and its vertical field of view.
type Camera struct {
	Width  int
	Height int
	VFOV   float64 // radians
}

func NewCamera(width, height int, vfov float64) Camera {
	return Camera{Width: width, Height: height, VFOV: vfov}
}

func (c Camera) focal() float64 {
	if c.VFOV <= 0 || c.VFOV >= math.Pi {
		return float64(c.Height)
	}
	return 0.5 * float64(c.Height) / math.Tan(c.VFOV/2)
}

// Ray returns a normalized world-space ray through the pixel center.
func (c Camera) Ray(px, py float64, pose Pose) Vec3 {
	f := c.focal()
	local := Vec3{
		X: (px - float64(c.Width)/2) / f,
		Y: -(py - float64(c.Height)/2) / f,
		Z: 1,
	}
	return rotateVec(local.Normalize(), pose.Yaw, pose.Pitch)
}

// Project returns the screen point and camera-space Z depth of a world point.
func (c Camera) Project(p Vec3, pose Pose) (Vec2, float64, bool) {
	cam := unrotateVec(p.Sub(pose.Position), pose.Yaw, pose.Pitch)
	if cam.Z <= nearClip {
		return Vec2{}, 0, false
	}
	f := c.focal()
	screen := Vec2{
		X: float64(c.Width)/2 + f*cam.X/cam.Z,
		Y: float64(c.Height)/2 - f*cam.Y/cam.Z,
	}
	return screen, cam.Z, true
}

// UnprojectScreen turns a screen point at a known camera-space depth into a
// world point under pose.
func (c Camera) UnprojectScreen(screen Vec2, depth float64, pose Pose) Vec3 {
	f := c.focal()
	local := Vec3{
		X: (screen.X - float64(c.Width)/2) / f * depth,
		Y: -(screen.Y - float64(c.Height)/2) / f * depth,
		Z: depth,
	}
	return rotateVec(local, pose.Yaw, pose.Pitch).Add(pose.Position)
}

func (c Camera) inside(screen Vec2) bool {
	return screen.X >= 0 && screen.Y >= 0 &&
		screen.X < float64(c.Width) && screen.Y < float64(c.Height)
}
