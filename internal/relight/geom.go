package relight

import (
	"math"
)

type Vec3 struct {
	X, Y, Z float64
}

func Add(a, b Vec3) Vec3 {
	return Vec3{a.X + b.X, a.Y + b.Y, a.Z + b.Z}
}

func Sub(a, b Vec3) Vec3 {
	return Vec3{a.X - b.X, a.Y - b.Y, a.Z - b.Z}
}

func Scale(a Vec3, s float64) Vec3 {
	return Vec3{a.X * s, a.Y * s, a.Z * s}
}

func Dot(a, b Vec3) float64 {
	return a.X*b.X + a.Y*b.Y + a.Z*b.Z
}

func Cross(a, b Vec3) Vec3 {
	return Vec3{
		X: a.Y*b.Z - a.Z*b.Y,
		Y: a.Z*b.X - a.X*b.Z,
		Z: a.X*b.Y - a.Y*b.X,
	}
}

func Length(a Vec3) float64 {
	return math.Sqrt(Dot(a, a))
}

func Normalize(a Vec3) Vec3 {
	norm := Length(a)
	if norm < 1e-12 {
		return Vec3{}
	}
	return Scale(a, 1/norm)
}

func Approx(a, b Vec3, tolerance float64) bool {
	return math.Abs(a.X-b.X) <= tolerance &&
		math.Abs(a.Y-b.Y) <= tolerance &&
		math.Abs(a.Z-b.Z) <= tolerance
}

type Pose struct {
	Pos     Vec3
	Right   Vec3
	Up      Vec3
	Forward Vec3
}

func LookAt(pos, target, worldUp Vec3) Pose {
	forward := Normalize(Sub(target, pos))
	right := Normalize(Cross(forward, worldUp))
	up := Cross(right, forward)
	return Pose{Pos: pos, Right: right, Up: up, Forward: forward}
}

type Camera struct {
	Pose     Pose
	Width    int
	Height   int
	Near     float64
	Far      float64
	TanHalfX float64
	TanHalfY float64
}

func NewCamera(pose Pose, width, height int, verticalFOVRadians, near, far float64) Camera {
	tanHalfY := math.Tan(verticalFOVRadians / 2)
	aspect := float64(width) / float64(height)
	return Camera{
		Pose:     pose,
		Width:    width,
		Height:   height,
		Near:     near,
		Far:      far,
		TanHalfX: tanHalfY * aspect,
		TanHalfY: tanHalfY,
	}
}

func (c Camera) Project(point Vec3) (pixelX, pixelY, depth float64, ok bool) {
	relative := Sub(point, c.Pose.Pos)
	depth = Dot(relative, c.Pose.Forward)
	if depth <= c.Near || depth >= c.Far {
		return 0, 0, 0, false
	}
	cameraX := Dot(relative, c.Pose.Right)
	cameraY := Dot(relative, c.Pose.Up)
	ndcX := cameraX / (depth * c.TanHalfX)
	ndcY := cameraY / (depth * c.TanHalfY)
	if ndcX < -1 || ndcX > 1 || ndcY < -1 || ndcY > 1 {
		return 0, 0, 0, false
	}
	pixelX = (ndcX + 1) * 0.5 * float64(c.Width)
	pixelY = (1 - ndcY) * 0.5 * float64(c.Height)
	return pixelX, pixelY, depth, true
}

func (c Camera) Unproject(pixelX, pixelY, depth float64) Vec3 {
	ndcX := 2*pixelX/float64(c.Width) - 1
	ndcY := 1 - 2*pixelY/float64(c.Height)
	viewVector := Add(
		Add(Scale(c.Pose.Right, ndcX*c.TanHalfX), Scale(c.Pose.Up, ndcY*c.TanHalfY)),
		c.Pose.Forward,
	)
	return Add(c.Pose.Pos, Scale(viewVector, depth))
}

func (c Camera) Ray(pixelX, pixelY float64) (origin, direction Vec3) {
	ndcX := 2*pixelX/float64(c.Width) - 1
	ndcY := 1 - 2*pixelY/float64(c.Height)
	direction = Normalize(Add(
		Add(Scale(c.Pose.Right, ndcX*c.TanHalfX), Scale(c.Pose.Up, ndcY*c.TanHalfY)),
		c.Pose.Forward,
	))
	return c.Pose.Pos, direction
}
