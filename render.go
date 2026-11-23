package main

import (
	"image"
	"image/color"
	"math"
)

// RenderEye ray-casts one eye view: objects are shaded, occluders are drawn
// as gray boxes (sheets slightly bluish to hint transparency).
func RenderEye(scene *Scene, rig *StereoRig, eye, width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	eyePos := rig.EyePos(eye)
	fwd, right, up := rig.Forward(), rig.Right(), rig.Up()
	halfH := math.Tan(rig.Fov / 2)
	halfW := halfH * float64(width) / float64(height)
	light := Norm(Vec3{0.4, 0.8, -0.3})
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			ndcX := (2*(float64(x)+0.5)/float64(width) - 1) * halfW
			ndcY := (1 - 2*(float64(y)+0.5)/float64(height)) * halfH
			dir := Norm(Add(fwd, Add(Mul(right, ndcX), Mul(up, ndcY))))
			ray := Ray{Orig: eyePos, Dir: dir}
			bestT := math.MaxFloat64
			var col [3]float64
			hitAny := false
			for _, obj := range scene.Objects {
				if t := raySphere(ray, obj.Center, obj.Radius); t > 0 && t < bestT {
					bestT = t
					hitAny = true
					p := Add(eyePos, Mul(dir, t))
					n := Norm(Sub(p, obj.Center))
					diff := 0.25 + 0.75*math.Max(0, Dot(n, light))
					col = [3]float64{obj.Color[0] * diff, obj.Color[1] * diff, obj.Color[2] * diff}
				}
			}
			for _, occ := range scene.Occluders {
				if t := rayBox(ray, occ.Min, occ.Max); t > 0 && t < bestT {
					bestT = t
					hitAny = true
					shade := 0.45
					if occ.Kind == Sheet {
						col = [3]float64{0.55, 0.65, 0.75}
					} else if occ.Kind == Pillar {
						col = [3]float64{shade, shade, shade + 0.05}
					} else {
						col = [3]float64{shade - 0.1, shade - 0.1, shade - 0.1}
					}
				}
			}
			if !hitAny {
				col = [3]float64{0.12, 0.14, 0.2}
			}
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(math.Min(col[0], 1) * 255),
				G: uint8(math.Min(col[1], 1) * 255),
				B: uint8(math.Min(col[2], 1) * 255),
				A: 255,
			})
		}
	}
	return img
}

// FrameDiff produces an amplified absolute-difference image between the two
// eye views (the stereo "picture difference").
func FrameDiff(a, b *image.RGBA) *image.RGBA {
	bounds := a.Bounds()
	out := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			ca := a.RGBAAt(x, y)
			cb := b.RGBAAt(x, y)
			d := func(u, v uint8) uint8 {
				diff := int(u) - int(v)
				if diff < 0 {
					diff = -diff
				}
				diff *= 3 // amplify for visibility
				if diff > 255 {
					diff = 255
				}
				return uint8(diff)
			}
			out.SetRGBA(x, y, color.RGBA{d(ca.R, cb.R), d(ca.G, cb.G), d(ca.B, cb.B), 255})
		}
	}
	return out
}
