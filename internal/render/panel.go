package render

import (
	"image"
	"image/color"
	"image/gif"

	"stereogaze/internal/gaze"
)

const (
	panelSize = 192
	headerH   = 14
	panelGap  = 4
)

const (
	idxBlack byte = iota
	idxWhite
	idxHeader
	idxPeripheral
	idxParafoveal
	idxFoveal
	idxOverlap
	idxShared
	idxBoundary
	idxOccluded
	idxStereoTint
	idxGaze
)

var paletteColors = color.Palette{
	idxBlack:      color.RGBA{0x10, 0x12, 0x18, 0xFF},
	idxWhite:      color.RGBA{0xEE, 0xEE, 0xEE, 0xFF},
	idxHeader:     color.RGBA{0x2A, 0x35, 0x48, 0xFF},
	idxPeripheral: color.RGBA{0x23, 0x2B, 0x38, 0xFF},
	idxParafoveal: color.RGBA{0x3D, 0x7E, 0xA6, 0xFF},
	idxFoveal:     color.RGBA{0xF2, 0xB1, 0x47, 0xFF},
	idxOverlap:    color.RGBA{0x6C, 0x5C, 0xE3, 0xFF},
	idxShared:     color.RGBA{0x35, 0xC4, 0x8C, 0xFF},
	idxBoundary:   color.RGBA{0xE0, 0x5A, 0x4F, 0xFF},
	idxOccluded:   color.RGBA{0x4A, 0x23, 0x36, 0xFF},
	idxStereoTint: color.RGBA{0x20, 0x3A, 0x44, 0xFF},
	idxGaze:       color.RGBA{0xFF, 0xFF, 0xFF, 0xFF},
}

// FrameArt is the five-panel stereoscopic frame.
type FrameArt struct {
	Image    *image.Paletted
	Delay100 int
}

// RenderFrame draws left/right sampling maps, overlap, shared mask and stereo.
func RenderFrame(left, right *gaze.Mask, report *gaze.Report, frameNumber int) *image.Paletted {
	width := 5*panelSize + 4*panelGap
	height := panelSize
	img := image.NewPaletted(image.Rect(0, 0, width, height), paletteColors)
	fill(img, idxBlack)

	originX := 0
	drawEyePanel(img, originX, left, "L", frameNumber)
	originX += panelSize + panelGap
	drawEyePanel(img, originX, right, "R", -1)
	originX += panelSize + panelGap
	drawOverlapPanel(img, originX, left, right, report)
	originX += panelSize + panelGap
	drawSharedPanel(img, originX, left, right, report)
	originX += panelSize + panelGap
	drawStereoPanel(img, originX, left, right)
	return img
}

// AnimatedGIF packs rendered frames into a looping, palette-only GIF.
func AnimatedGIF(frames []*image.Paletted, delay100 int) *gif.GIF {
	animated := &gif.GIF{LoopCount: 0}
	for _, frame := range frames {
		animated.Image = append(animated.Image, frame)
		animated.Delay = append(animated.Delay, delay100)
	}
	return animated
}

func drawEyePanel(img *image.Paletted, x int, mask *gaze.Mask, label string, frameNumber int) {
	drawHeader(img, x, 0, label)
	if frameNumber >= 0 {
		drawHeaderText(img, x+panelSize-textWidth(frameLabel(frameNumber), 1)-4, 1, frameLabel(frameNumber))
	}
	if mask.Occluded {
		fillRect(img, x, headerH, panelSize, panelSize-headerH, idxOccluded)
		drawCenteredText(img, x, "OCCL")
		return
	}
	drawMaskGrid(img, x, headerH, mask)
	drawBoundary(img, x, headerH, mask, idxBoundary)
	drawGazeMarker(img, x, headerH, panelSize, mask.Gaze)
}

func drawOverlapPanel(img *image.Paletted, x int, left, right *gaze.Mask, report *gaze.Report) {
	drawHeader(img, x, 0, "O")
	drawMaskGrid(img, x, headerH, left)
	for row := 0; row < report.Size; row++ {
		for col := 0; col < report.Size; col++ {
			if report.Overlap[row][col] {
				setGridCell(img, x, headerH, report.Size, row, col, idxOverlap)
			}
		}
	}
	drawGazeMarker(img, x, headerH, panelSize, left.Gaze)
	drawGazeMarker(img, x, headerH, panelSize, right.Gaze)
}

func drawSharedPanel(img *image.Paletted, x int, left, right *gaze.Mask, report *gaze.Report) {
	drawHeader(img, x, 0, "S")
	drawMaskGrid(img, x, headerH, left)
	for row := 0; row < report.Size; row++ {
		for col := 0; col < report.Size; col++ {
			if report.Shared[row][col] {
				setGridCell(img, x, headerH, report.Size, row, col, idxShared)
			}
		}
	}
	drawGazeMarker(img, x, headerH, panelSize, left.Gaze)
	drawGazeMarker(img, x, headerH, panelSize, right.Gaze)
}

func drawStereoPanel(img *image.Paletted, x int, left, right *gaze.Mask) {
	drawHeader(img, x, 0, "ST")
	halfWidth := panelSize / 2
	drawStereoHalf(img, x, headerH, halfWidth, panelSize-headerH, left, "L")
	drawStereoHalf(img, x+halfWidth, headerH, halfWidth, panelSize-headerH, right, "R")
	for y := headerH; y < panelSize; y++ {
		img.SetColorIndex(x+halfWidth, y, idxWhite)
	}
}

func drawStereoHalf(img *image.Paletted, x, y, width, height int, mask *gaze.Mask, label string) {
	fillRect(img, x, y, width, height, idxStereoTint)
	for row := 0; row < mask.Size; row++ {
		for col := 0; col < mask.Size; col++ {
			cellX := x + col*width/mask.Size
			cellY := y + row*height/mask.Size
			nextX := x + (col+1)*width/mask.Size
			nextY := y + (row+1)*height/mask.Size
			fillRect(img, cellX, cellY, nextX-cellX, nextY-cellY, levelColor(mask.Tiers[row][col], mask.Occluded))
		}
	}
	gazeX := x + int(mask.Gaze.X*float64(width))
	gazeY := y + int(mask.Gaze.Y*float64(height))
	drawCross(img, gazeX, gazeY)
	drawHeaderText(img, x+3, headerH+2, label)
}

func drawMaskGrid(img *image.Paletted, x, y int, mask *gaze.Mask) {
	for row := 0; row < mask.Size; row++ {
		for col := 0; col < mask.Size; col++ {
			setGridCell(img, x, y, mask.Size, row, col, levelColor(mask.Tiers[row][col], mask.Occluded))
		}
	}
}

func drawBoundary(img *image.Paletted, x, y int, mask *gaze.Mask, colorIndex byte) {
	for row := 0; row < mask.Size; row++ {
		for col := 0; col < mask.Size; col++ {
			if mask.Boundary[row][col] {
				setGridCell(img, x, y, mask.Size, row, col, colorIndex)
			}
		}
	}
}

func setGridCell(img *image.Paletted, originX, originY, size, row, col int, colorIndex byte) {
	x := originX + col*panelSize/size
	y := originY + row*(panelSize-headerH)/size
	width := (col+1)*panelSize/size - col*panelSize/size
	height := (row+1)*(panelSize-headerH)/size - row*(panelSize-headerH)/size
	fillRect(img, x, y, width, height, colorIndex)
}

func drawGazeMarker(img *image.Paletted, originX, originY, dimension int, gaze gaze.Point) {
	x := originX + int(gaze.X*float64(dimension))
	y := originY + int(gaze.Y*float64(dimension-headerH))
	drawCross(img, x, y)
}

func drawCross(img *image.Paletted, x, y int) {
	for dx := -3; dx <= 3; dx++ {
		safeSet(img, x+dx, y, idxGaze)
		safeSet(img, x, y+dx, idxGaze)
	}
}

func safeSet(img *image.Paletted, x, y int, colorIndex byte) {
	bounds := img.Bounds()
	if x < bounds.Min.X || x >= bounds.Max.X || y < bounds.Min.Y || y >= bounds.Max.Y {
		return
	}
	img.SetColorIndex(x, y, colorIndex)
}

func drawHeader(img *image.Paletted, x, y int, label string) {
	fillRect(img, x, y, panelSize, headerH, idxHeader)
	drawHeaderText(img, x+4, 1, label)
}

func drawHeaderText(img *image.Paletted, x, y int, text string) {
	drawText(img.Pix, img.Stride, x, y, 1, text, idxWhite)
}

func drawCenteredText(img *image.Paletted, x int, text string) {
	textX := x + (panelSize-textWidth(text, 2))/2
	textY := headerH + (panelSize-headerH-glyphHeight*2)/2
	drawText(img.Pix, img.Stride, textX, textY, 2, text, idxWhite)
}

func frameLabel(frame int) string {
	return "F:" + itoa(frame)
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var digits []byte
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}

func levelColor(level gaze.Level, occluded bool) byte {
	if occluded {
		return idxOccluded
	}
	switch level {
	case gaze.Foveal:
		return idxFoveal
	case gaze.Parafoveal:
		return idxParafoveal
	default:
		return idxPeripheral
	}
}

func fillRect(img *image.Paletted, x, y, width, height int, colorIndex byte) {
	for row := y; row < y+height; row++ {
		for col := x; col < x+width; col++ {
			img.SetColorIndex(col, row, colorIndex)
		}
	}
}

func fill(img *image.Paletted, colorIndex byte) {
	for index := range img.Pix {
		img.Pix[index] = colorIndex
	}
}
