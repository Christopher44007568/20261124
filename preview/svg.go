package preview

import (
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"strings"

	"headpose"
)

// RenderSVG writes a standalone, dependency-free SVG preview.
func RenderSVG(out io.Writer, input Preview, options Options) error {
	data, err := prepare(input, options)
	if err != nil {
		return err
	}
	layout := makeLayout(data.options)
	worldFit := worldFitPoints(data)
	worldProjection := fitWorldProjector(worldFit, layout.worldBounds)
	screenProjection := fitScreenProjector(
		data.options.ScreenWidthPixels,
		data.options.ScreenHeightPixels,
		layout.screenBounds,
	)

	doc := &document{}
	doc.f(`<?xml version="1.0" encoding="UTF-8"?>`)
	doc.f(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" role="img" aria-labelledby="preview-title preview-desc">`,
		data.options.Width, data.options.Height, data.options.Width, data.options.Height)
	doc.f(`<title id="preview-title">%s</title>`, xmlEscape(data.options.Title))
	doc.f(`<desc id="preview-desc">Anchors, axis arrows, camera trajectory, and a reversible pixel-screen mapping.</desc>`)
	writeDefs(doc)
	writeBackground(doc, data, layout)
	writeWorldPanel(doc, data, layout, worldProjection)
	writeScreenPanel(doc, data, layout, screenProjection)
	doc.f(`</svg>`)

	_, err = io.WriteString(out, doc.String())
	return err
}

type document struct {
	builder strings.Builder
}

func (d *document) f(format string, args ...any) {
	fmt.Fprintf(&d.builder, format, args...)
}

func (d *document) raw(value string) {
	d.builder.WriteString(value)
}

func (d *document) String() string {
	return d.builder.String()
}

type layout struct {
	worldPanel   rect
	screenPanel  rect
	worldBounds  rect
	screenBounds rect
}

func makeLayout(options Options) layout {
	const margin = 32.0
	const gap = 24.0
	const panelTop = 110.0
	const panelBottom = 32.0

	rightWidth := math.Round(float64(options.Width) * 0.285)
	rightWidth = math.Max(280, math.Min(360, rightWidth))
	panelHeight := float64(options.Height) - panelTop - panelBottom
	leftWidth := float64(options.Width) - 2*margin - gap - rightWidth

	worldPanel := rect{X: margin, Y: panelTop, Width: leftWidth, Height: panelHeight}
	screenPanel := rect{
		X:      margin + leftWidth + gap,
		Y:      panelTop,
		Width:  rightWidth,
		Height: panelHeight,
	}
	return layout{
		worldPanel:  worldPanel,
		screenPanel: screenPanel,
		worldBounds: rect{
			X:      worldPanel.X + 24,
			Y:      worldPanel.Y + 64,
			Width:  worldPanel.Width - 48,
			Height: worldPanel.Height - 92,
		},
		screenBounds: rect{
			X:      screenPanel.X + 20,
			Y:      screenPanel.Y + 70,
			Width:  screenPanel.Width - 40,
			Height: 190,
		},
	}
}

func worldFitPoints(data preparedPreview) []headpose.Vec3 {
	points := []headpose.Vec3{
		{X: -2.2, Z: -2.2},
		{X: 2.2, Z: -2.2},
		{X: -2.2, Z: 2.2},
		{X: 2.2, Z: 2.2},
	}
	points = append(points, axisTripodPoints(headpose.Vec3{}, headpose.IdentityQuat(), 0.7)...)
	devicePose := data.scene.DevicePoseInWorld()
	points = append(points, axisTripodPoints(devicePose.Translation(), devicePose.Rotation(), 0.42)...)
	cameraPose := data.scene.CameraPoseInWorld()
	points = append(points, axisTripodPoints(cameraPose.Translation(), cameraPose.Rotation(), 0.42)...)
	for _, anchor := range data.anchors {
		points = append(points, anchor.Position)
	}
	for _, sample := range data.trajectory {
		position := sample.Translation()
		forward := sample.ApplyDirection(headpose.Vec3{Z: -1})
		points = append(points, position, position.Add(forward.Scale(0.55)))
	}
	return points
}

func axisTripodPoints(origin headpose.Vec3, rotation headpose.Quat, length float64) []headpose.Vec3 {
	return []headpose.Vec3{
		origin,
		origin.Add(rotation.Rotate(headpose.Vec3{X: 1}).Scale(length)),
		origin.Add(rotation.Rotate(headpose.Vec3{Y: 1}).Scale(length)),
		origin.Add(rotation.Rotate(headpose.Vec3{Z: 1}).Scale(length)),
	}
}

func writeDefs(doc *document) {
	doc.raw(`<defs>`)
	doc.raw(`<linearGradient id="panel-fill" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#111a2e"/><stop offset="1" stop-color="#0b1324"/></linearGradient>`)
	doc.raw(`<filter id="soft-glow" x="-80%" y="-80%" width="260%" height="260%"><feGaussianBlur stdDeviation="3"/></filter>`)
	doc.raw(`</defs>`)
}

func writeBackground(doc *document, data preparedPreview, layout layout) {
	doc.raw(`<rect width="100%" height="100%" fill="#080d18"/>`)
	doc.f(`<text x="32" y="43" fill="#ffffff" font-family="ui-sans-serif, system-ui, sans-serif" font-size="24" font-weight="600">%s</text>`,
		xmlEscape(data.options.Title))
	doc.raw(`<text x="32" y="67" fill="#8fa6c6" font-family="ui-sans-serif, system-ui, sans-serif" font-size="12">World + Device + Camera in metres · Screen in pixels · quaternions WXYZ · active right-handed rotations</text>`)
	doc.raw(`<text x="32" y="88" fill="#536b8c" font-family="ui-monospace, SFMono-Regular, monospace" font-size="11">q and -q are equivalent · Screen Z is pixel depth · path: Device → World → Camera → Screen</text>`)

	for _, panel := range []rect{layout.worldPanel, layout.screenPanel} {
		doc.f(`<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="14" fill="url(#panel-fill)" stroke="#223452"/>`,
			panel.X, panel.Y, panel.Width, panel.Height)
	}
	doc.f(`<text x="%.1f" y="%.1f" fill="#dce9ff" font-family="ui-sans-serif, system-ui, sans-serif" font-size="14" font-weight="600">World / isometric</text>`,
		layout.worldPanel.X+22, layout.worldPanel.Y+34)
	doc.f(`<text x="%.1f" y="%.1f" fill="#6f87a8" font-family="ui-monospace, SFMono-Regular, monospace" font-size="10">+X east · +Y up · +Z south · metres</text>`,
		layout.worldPanel.X+22, layout.worldPanel.Y+51)
	doc.f(`<text x="%.1f" y="%.1f" fill="#dce9ff" font-family="ui-sans-serif, system-ui, sans-serif" font-size="14" font-weight="600">Screen / pixels</text>`,
		layout.screenPanel.X+20, layout.screenPanel.Y+34)
	doc.f(`<text x="%.1f" y="%.1f" fill="#6f87a8" font-family="ui-monospace, SFMono-Regular, monospace" font-size="10">+X right · +Y down · +Z depth px</text>`,
		layout.screenPanel.X+20, layout.screenPanel.Y+51)
}

func xmlEscape(value string) string {
	var builder strings.Builder
	if err := xml.EscapeText(&builder, []byte(value)); err != nil {
		return ""
	}
	return builder.String()
}

func pointString(p point) string {
	return fmt.Sprintf("%.2f,%.2f", p.X, p.Y)
}

func writeArrow(
	doc *document,
	from, to point,
	color string,
	width, opacity float64,
) {
	doc.f(`<line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="%s" stroke-width="%.1f" stroke-opacity="%.2f" stroke-linecap="round"/>`,
		from.X, from.Y, to.X, to.Y, color, width, opacity)
	dx, dy := to.X-from.X, to.Y-from.Y
	length := math.Hypot(dx, dy)
	if length < 0.1 {
		return
	}
	ux, uy := dx/length, dy/length
	headLength := math.Min(8, length*0.6)
	halfWidth := headLength * 0.42
	baseX, baseY := to.X-ux*headLength, to.Y-uy*headLength
	px, py := -uy*halfWidth, ux*halfWidth
	doc.f(`<polygon class="arrow-head" points="%.2f,%.2f %.2f,%.2f %.2f,%.2f" fill="%s" fill-opacity="%.2f"/>`,
		to.X, to.Y, baseX+px, baseY+py, baseX-px, baseY-py, color, opacity)
}
