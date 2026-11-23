package stereo

import (
	"math"
	"strconv"
)

var (
	wallBase  = RGB{R: 0.71, G: 0.72, B: 0.68}
	rulerBase = RGB{R: 0.92, G: 0.90, B: 0.80}
	sphereCol = RGB{R: 0.16, G: 0.55, B: 0.27}
	frontCol  = RGB{R: 0.86, G: 0.38, B: 0.13}
	backCol   = RGB{R: 0.14, G: 0.38, B: 0.74}
	inkCol    = RGB{R: 0.08, G: 0.07, B: 0.06}
	redCol    = RGB{R: 0.70, G: 0.10, B: 0.12}
)

func Shade(hit Hit) RGB {
	switch hit.Surface {
	case SurfaceRuler:
		if c, ok := rulerColor(hit.Point); ok {
			return litColor(c, hit, 0x51c3)
		}
		return litColor(wallBase, hit, 0x1111)
	case SurfaceSphere:
		return litColor(sphereCol, hit, 0x2201)
	case SurfaceFrontPlate:
		c := plateColor(frontCol, hit.Point, FrontPlateMin, FrontPlateMax)
		return litColor(c, hit, 0x3301)
	case SurfaceBackPlate:
		c := plateColor(backCol, hit.Point, BackPlateMin, BackPlateMax)
		return litColor(c, hit, 0x4401)
	default:
		return litColor(wallBase, hit, 0x1101)
	}
}

func litColor(base RGB, hit Hit, seed uint64) RGB {
	grain := 0.80 + 0.40*valueNoise(hit.Point, 6.5, seed)
	fine := 0.92 + 0.16*valueNoise(hit.Point, 2.6, seed+17)
	light := Vec3{X: -0.42, Y: 0.72, Z: -0.56}.Normalize()
	diffuse := math.Max(hit.Normal.Dot(light), 0)
	illumination := 0.58 + 0.55*diffuse
	return base.Scale(grain * fine * illumination)
}

func plateColor(base RGB, p Vec3, min, max Vec3) RGB {
	border := 3.0
	if p.X-min.X < border || max.X-p.X < border || p.Y-min.Y < border || max.Y-p.Y < border {
		return base.Scale(0.52)
	}
	return base
}

func rulerColor(p Vec3) (RGB, bool) {
	if p.X < RulerMinX || p.X > RulerMaxX || p.Y < RulerBotY || p.Y > RulerTopY {
		return RGB{}, false
	}

	base := rulerBase.Scale(0.96 + 0.08*valueNoise(p, 7.5, 0x9a01))
	if p.Y <= RulerBotY+3 || p.Y >= RulerTopY-3 ||
		p.X <= RulerMinX+3 || p.X >= RulerMaxX-3 {
		return inkCol, true
	}

	if c, ok := rulerLabel(p); ok {
		return c, true
	}

	const tickStep = 10.0
	k := math.Round(p.X / tickStep)
	x := k * tickStep
	if math.Abs(p.X-x) <= 2.0 {
		length := 24.0
		c := inkCol
		switch {
		case math.Mod(math.Abs(k), 10) == 0:
			length = 66
			c = redCol
		case math.Mod(math.Abs(k), 5) == 0:
			length = 48
			c = redCol
		}
		if p.Y >= RulerTopY-length && p.Y <= RulerTopY {
			return c, true
		}
	}
	return base, true
}

func rulerLabel(p Vec3) (RGB, bool) {
	label := int(math.Round(p.X/100.0)) * 100
	if label < -400 || label > 400 {
		return RGB{}, false
	}
	text := strconv.Itoa(label)
	const cell = 6.0
	const glyphW = 3 * cell
	const advance = 4 * cell
	width := float64(len(text))*advance - cell
	textTop := -214.0
	left := float64(label) - width/2
	local := p.X - left
	if local < 0 || local >= width {
		return RGB{}, false
	}
	charIndex := int(local / advance)
	if charIndex >= len(text) {
		return RGB{}, false
	}
	within := local - float64(charIndex)*advance
	if within >= glyphW {
		return RGB{}, false
	}
	row := int(math.Floor((textTop - p.Y) / cell))
	if row < 0 || row >= 5 {
		return RGB{}, false
	}
	col := int(math.Floor(within / cell))
	glyph := glyph3x5(text[charIndex])
	if glyph[row]&(1<<uint(2-col)) != 0 {
		return inkCol, true
	}
	return RGB{}, false
}

func glyph3x5(ch byte) [5]uint8 {
	switch ch {
	case '0':
		return [5]uint8{7, 5, 5, 5, 7}
	case '1':
		return [5]uint8{2, 6, 2, 2, 7}
	case '2':
		return [5]uint8{7, 1, 7, 4, 7}
	case '3':
		return [5]uint8{7, 1, 7, 1, 7}
	case '4':
		return [5]uint8{5, 5, 7, 1, 1}
	case '5':
		return [5]uint8{7, 4, 7, 1, 7}
	case '6':
		return [5]uint8{7, 4, 7, 5, 7}
	case '7':
		return [5]uint8{7, 1, 2, 2, 2}
	case '8':
		return [5]uint8{7, 5, 7, 5, 7}
	case '9':
		return [5]uint8{7, 5, 7, 1, 7}
	case '-':
		return [5]uint8{0, 0, 7, 0, 0}
	default:
		return [5]uint8{}
	}
}
