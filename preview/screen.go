package preview

import (
	"math"
	"strconv"

	"headpose"
)

func writeScreenPanel(doc *document, data preparedPreview, layout layout, projection screenProjector) {
	viewport := projection.viewport()
	params := data.scene.Screen()
	doc.f(`<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="6" fill="#060a13" stroke="#2d4668"/>`,
		viewport.X, viewport.Y, viewport.Width, viewport.Height)
	doc.f(`<clipPath id="screen-clip"><rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="5"/></clipPath>`,
		viewport.X, viewport.Y, viewport.Width, viewport.Height)
	doc.f(`<g clip-path="url(#screen-clip)">`)
	writePixelGrid(doc, data.options, projection)
	writePrincipalPoint(doc, params, projection)
	for _, anchor := range data.anchors {
		screen, err := data.scene.ConvertPoint(headpose.World, headpose.Screen, anchor.Position)
		if err != nil || !insideViewport(screen, data.options) {
			continue
		}
		writeScreenAnchor(doc, anchor, screen, projection)
	}
	writeScreenAxes(doc, viewport)
	doc.f(`</g>`)
	writeScreenSummary(doc, data, layout, projection)
}

func writePixelGrid(doc *document, options Options, projection screenProjector) {
	viewport := projection.viewport()
	for i := 1; i < 4; i++ {
		x := viewport.X + viewport.Width*float64(i)/4
		doc.f(`<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#101d31" stroke-width="0.8"/>`,
			x, viewport.Y, x, viewport.Y+viewport.Height)
	}
	for i := 1; i < 3; i++ {
		y := viewport.Y + viewport.Height*float64(i)/3
		doc.f(`<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#101d31" stroke-width="0.8"/>`,
			viewport.X, y, viewport.X+viewport.Width, y)
	}
	doc.f(`<text x="%.1f" y="%.1f" fill="#3f5879" font-family="ui-monospace, SFMono-Regular, monospace" font-size="8">0,0</text>`,
		viewport.X+5, viewport.Y+11)
	doc.f(`<text x="%.1f" y="%.1f" fill="#3f5879" font-family="ui-monospace, SFMono-Regular, monospace" font-size="8" text-anchor="end">%d,%d</text>`,
		viewport.X+viewport.Width-5, viewport.Y+viewport.Height-5,
		options.ScreenWidthPixels, options.ScreenHeightPixels)
}

func writePrincipalPoint(doc *document, params headpose.ScreenParams, projection screenProjector) {
	principal := projection.project(headpose.Vec3{X: params.PrincipalX, Y: params.PrincipalY})
	doc.f(`<line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="#3a5478" stroke-width="0.9" stroke-dasharray="3 3"/>`,
		principal.X-10, principal.Y, principal.X+10, principal.Y)
	doc.f(`<line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="#3a5478" stroke-width="0.9" stroke-dasharray="3 3"/>`,
		principal.X, principal.Y-10, principal.X, principal.Y+10)
	doc.f(`<circle cx="%.2f" cy="%.2f" r="2.4" fill="#8aa0bd"/>`, principal.X, principal.Y)
}

func writeScreenAnchor(
	doc *document,
	anchor Anchor,
	screen headpose.Vec3,
	projection screenProjector,
) {
	p := projection.project(screen)
	doc.f(`<circle cx="%.2f" cy="%.2f" r="9" fill="%s" opacity="0.20"/>`, p.X, p.Y, anchor.Color)
	doc.f(`<circle cx="%.2f" cy="%.2f" r="4.8" fill="%s" stroke="#f4f8ff" stroke-width="1"/>`,
		p.X, p.Y, anchor.Color)
	doc.f(`<text x="%.2f" y="%.2f" fill="%s" font-family="ui-sans-serif, system-ui, sans-serif" font-size="10" font-weight="650" stroke="#060a13" stroke-width="3" paint-order="stroke">%s</text>`,
		p.X+7, p.Y-6, anchor.Color, xmlEscape(anchor.Label))
}

func writeScreenAxes(doc *document, viewport rect) {
	x := viewport.X + 13
	y := viewport.Y + viewport.Height - 16
	writeArrow(doc, point{X: x, Y: y}, point{X: x + 25, Y: y}, "#ff6b6b", 1.6, 1)
	writeArrow(doc, point{X: x, Y: y}, point{X: x, Y: y + 20}, "#7ee787", 1.6, 1)
	doc.f(`<text x="%.1f" y="%.1f" fill="#7c94b3" font-family="ui-monospace, SFMono-Regular, monospace" font-size="8">+X</text>`,
		x+27, y+3)
	doc.f(`<text x="%.1f" y="%.1f" fill="#7c94b3" font-family="ui-monospace, SFMono-Regular, monospace" font-size="8">+Y</text>`,
		x-9, y+30)
	doc.f(`<text x="%.1f" y="%.1f" fill="#6aa9ff" font-family="ui-monospace, SFMono-Regular, monospace" font-size="8">+Z into screen</text>`,
		x+44, y+11)
}

func writeScreenSummary(
	doc *document,
	data preparedPreview,
	layout layout,
	projection screenProjector,
) {
	panel := layout.screenPanel
	viewport := projection.viewport()
	params := data.scene.Screen()
	y := viewport.Y + viewport.Height + 34
	doc.f(`<text x="%.1f" y="%.1f" fill="#8fa6c6" font-family="ui-monospace, SFMono-Regular, monospace" font-size="9">%d × %d px · %.1f px/m</text>`,
		panel.X+20, y, data.options.ScreenWidthPixels, data.options.ScreenHeightPixels, params.PixelsPerMeter)
	doc.f(`<text x="%.1f" y="%.1f" fill="#8fa6c6" font-family="ui-monospace, SFMono-Regular, monospace" font-size="9">principal (%.1f, %.1f) px</text>`,
		panel.X+20, y+15, params.PrincipalX, params.PrincipalY)
	doc.f(`<text x="%.1f" y="%.1f" fill="#536b8c" font-family="ui-monospace, SFMono-Regular, monospace" font-size="8">affine inverse keeps depth · Z = px</text>`,
		panel.X+20, y+29)
	doc.f(`<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#1d2e49"/>`,
		panel.X+20, y+38, panel.X+panel.Width-20, y+38)

	rowY := y + 58
	doc.f(`<text x="%.1f" y="%.1f" fill="#607899" font-family="ui-sans-serif, system-ui, sans-serif" font-size="9" letter-spacing="0.6">WORLD → SCREEN ANCHORS</text>`,
		panel.X+20, rowY)
	rowY += 20
	for _, anchor := range data.anchors {
		if rowY > panel.Y+panel.Height-18 {
			break
		}
		screen, err := data.scene.ConvertPoint(headpose.World, headpose.Screen, anchor.Position)
		doc.f(`<circle cx="%.1f" cy="%.1f" r="3.2" fill="%s"/>`, panel.X+25, rowY-3, anchor.Color)
		doc.f(`<text x="%.1f" y="%.1f" fill="#c9d8ec" font-family="ui-sans-serif, system-ui, sans-serif" font-size="9">%s</text>`,
			panel.X+35, rowY, xmlEscape(anchor.Label))
		value := "unavailable"
		if err == nil {
			value = "(" + formatPixel(screen.X) + ", " + formatPixel(screen.Y) + ")"
			if !insideViewport(screen, data.options) {
				value += " outside"
			}
		}
		doc.f(`<text x="%.1f" y="%.1f" fill="#7d93b1" font-family="ui-monospace, SFMono-Regular, monospace" font-size="8.5" text-anchor="end">%s</text>`,
			panel.X+panel.Width-20, rowY, xmlEscape(value))
		rowY += 17
	}
}

func insideViewport(screen headpose.Vec3, options Options) bool {
	return screen.X >= 0 && screen.X <= float64(options.ScreenWidthPixels) &&
		screen.Y >= 0 && screen.Y <= float64(options.ScreenHeightPixels)
}

func formatPixel(value float64) string {
	return strconv.FormatFloat(math.Round(value), 'f', 0, 64)
}
