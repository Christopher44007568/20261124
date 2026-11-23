package shadow

import "math"

const (
	litGround uint8 = iota
	shadowGround
	sky
)

// RenderEye renders the ground shadow classification seen by one eye.
// sample receives world-space ground points and returns true for shadow.
func RenderEye(scene Scene, eye Eye, width, height int,
	sample func(Vec2) bool) []uint8 {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	img := make([]uint8, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			p, ok := groundPoint(scene, eye, x, y, width, height)
			if !ok {
				img[y*width+x] = sky
				continue
			}
			if sample(p) {
				img[y*width+x] = shadowGround
			} else {
				img[y*width+x] = litGround
			}
		}
	}
	return img
}

func groundPoint(scene Scene, eye Eye, x, y, width, height int) (Vec2, bool) {
	sx := (2*((float64(x)+0.5)/float64(width)) - 1) * math.Tan(eye.HFOV*0.5)
	sy := (1 - 2*((float64(y)+0.5)/float64(height))) * math.Tan(eye.VFOV*0.5)
	forward := eye.forward()
	right := Vec3{X: math.Cos(eye.Yaw), Z: -math.Sin(eye.Yaw)}
	up := Vec3{
		X: forward.Y*right.Z - forward.Z*right.Y,
		Y: forward.Z*right.X - forward.X*right.Z,
		Z: forward.X*right.Y - forward.Y*right.X,
	}
	dir := forward.Add(right.Scale(sx)).Add(up.Scale(sy)).Normalized()
	if dir.Y >= -eps {
		return Vec2{}, false
	}
	t := -eye.Pos.Y / dir.Y
	if t <= 0 {
		return Vec2{}, false
	}
	return Vec2{X: eye.Pos.X + dir.X*t, Z: eye.Pos.Z + dir.Z*t}, true
}

// ReferenceShadows renders the analytic, unquantized shadow result.
func ReferenceShadows(scene Scene, masks []EyeMask, light Vec3, eye Eye,
	width, height int) []uint8 {
	return RenderEye(scene, eye, width, height, func(p Vec2) bool {
		return shadowAt(scene, masks, light, p, eyeBit(eye.Side))
	})
}

// StrategyShadows renders the quantized shadow-map result for one eye.
func StrategyShadows(scene Scene, store *ShadowStore, eye Eye,
	width, height int) []uint8 {
	bit := eyeBit(eye.Side)
	return RenderEye(scene, eye, width, height, func(p Vec2) bool {
		return store.Sample(p, bit)
	})
}

func eyeBit(side float64) EyeMask {
	if side < 0 {
		return LeftEye
	}
	return RightEye
}

// ImageError counts shadow-classification mismatches on pixels where both
// images reached the ground.
type ImageError struct {
	Compared int
	Mismatch int
}

func (e ImageError) Rate() float64 {
	if e.Compared == 0 {
		return 0
	}
	return float64(e.Mismatch) / float64(e.Compared)
}

func CompareImages(reference, actual []uint8) ImageError {
	var out ImageError
	n := len(reference)
	if len(actual) < n {
		n = len(actual)
	}
	for i := 0; i < n; i++ {
		if reference[i] == sky || actual[i] == sky {
			continue
		}
		out.Compared++
		if reference[i] != actual[i] {
			out.Mismatch++
		}
	}
	return out
}
