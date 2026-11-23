package replay

import (
	"bytes"
	"encoding/hex"
	"image"
	"image/color"
	"image/png"
	"math"
)

const (
	frameWidth  = 192
	frameHeight = 108
	eyeWidth    = frameWidth / 2
	stampHeight = 10
)

func renderFramePNG(matrix *MatrixRecord) ([]byte, error) {
	img := image.NewNRGBA(image.Rect(0, 0, frameWidth, frameHeight))
	drawEye(img, image.Rect(0, 0, eyeWidth, frameHeight), eyeVisualFor(matrix, EyeLeft))
	drawEye(img, image.Rect(eyeWidth, 0, frameWidth, frameHeight), eyeVisualFor(matrix, EyeRight))

	for y := 0; y < stampHeight; y++ {
		for x := 0; x < frameWidth; x++ {
			img.SetNRGBA(x, y, color.NRGBA{A: 255})
		}
	}

	matrixVersion, leftVersion, rightVersion := uint16(0), uint16(0), uint16(0)
	if matrix != nil {
		matrixVersion = uint16(matrix.Version)
		leftVersion = uint16(matrix.LeftPoseVersion)
		rightVersion = uint16(matrix.RightPoseVersion)
	}
	drawVersionStamp(img, 0, matrixVersion)
	drawVersionStamp(img, 64, leftVersion)
	drawVersionStamp(img, 128, rightVersion)

	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

type eyeVisual struct {
	yaw, pitch, roll float64
	digest           [8]byte
	side             Eye
}

func eyeVisualFor(matrix *MatrixRecord, eye Eye) eyeVisual {
	visual := eyeVisual{side: eye}
	if matrix == nil {
		return visual
	}
	if eye == EyeLeft {
		visual.yaw = matrix.LeftYawDeg
		visual.pitch = matrix.LeftPitchDeg
		visual.roll = matrix.LeftRollDeg
	} else {
		visual.yaw = matrix.RightYawDeg
		visual.pitch = matrix.RightPitchDeg
		visual.roll = matrix.RightRollDeg
	}
	raw, _ := hex.DecodeString(matrix.Digest[:16])
	copy(visual.digest[:], raw)
	return visual
}

func drawEye(img *image.NRGBA, bounds image.Rectangle, visual eyeVisual) {
	horizon := bounds.Min.Y + frameHeight/2 + int(math.Round(visual.pitch*0.9))
	if horizon < bounds.Min.Y+stampHeight+4 {
		horizon = bounds.Min.Y + stampHeight + 4
	}
	if horizon > bounds.Max.Y-4 {
		horizon = bounds.Max.Y - 4
	}
	shift := int(math.Round(visual.yaw * 1.7))
	accent := color.NRGBA{
		R: 80 + visual.digest[0]/2,
		G: 80 + visual.digest[1]/2,
		B: 80 + visual.digest[2]/2,
		A: 255,
	}

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			var pixel color.NRGBA
			if y < horizon {
				level := uint8((y - bounds.Min.Y) * 45 / frameHeight)
				pixel = color.NRGBA{R: 25 + level, G: 55 + level, B: 105 + level, A: 255}
			} else {
				level := uint8((y - horizon) * 35 / frameHeight)
				pixel = color.NRGBA{R: 55 + level, G: 95 + level/2, B: 55, A: 255}
			}
			if visual.side == EyeLeft && (x+y)%17 == 0 {
				pixel.R = blend(pixel.R, 150, 30)
			}
			if visual.side == EyeRight && (x+y)%17 == 0 {
				pixel.B = blend(pixel.B, 150, 30)
			}
			img.SetNRGBA(x, y, pixel)
		}
	}

	poleX := bounds.Min.X + eyeWidth/2 + shift
	for y := horizon - 30; y < horizon+18; y++ {
		for x := poleX - 2; x <= poleX+2; x++ {
			if image.Pt(x, y).In(bounds) {
				img.SetNRGBA(x, y, accent)
			}
		}
	}
	for x := bounds.Min.X; x < bounds.Max.X; x++ {
		if x >= poleX-18 && x <= poleX+18 {
			continue
		}
		img.SetNRGBA(x, horizon, color.NRGBA{R: 230, G: 220, B: 190, A: 255})
	}
	drawMarker(img, bounds, poleX, horizon-30, visual.digest)
}

func drawMarker(img *image.NRGBA, bounds image.Rectangle, x, y int, digest [8]byte) {
	size := 6 + int(digest[3]%5)
	for dy := 0; dy < size; dy++ {
		for dx := 0; dx < size; dx++ {
			pixel := image.Pt(x-size/2+dx, y-size+dy)
			if pixel.In(bounds) && pixel.Y > stampHeight {
				img.SetNRGBA(pixel.X, pixel.Y, color.NRGBA{
					R: 180 + digest[4]/4,
					G: 70 + digest[5]/4,
					B: 50 + digest[6]/4,
					A: 255,
				})
			}
		}
	}
}

func drawVersionStamp(img *image.NRGBA, x0 int, value uint16) {
	for bit := 0; bit < 16; bit++ {
		shade := uint8(32)
		if value&(1<<bit) != 0 {
			shade = 245
		}
		for y := 1; y < stampHeight-1; y++ {
			for x := x0 + bit*4 + 1; x < x0+bit*4+3; x++ {
				img.SetNRGBA(x, y, color.NRGBA{R: shade, G: shade, B: shade, A: 255})
			}
		}
	}
}

func blend(base, overlay uint8, weight int) uint8 {
	return uint8((int(base)*(100-weight) + int(overlay)*weight) / 100)
}
