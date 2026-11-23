package preview

import "headpose"

func writeWorldPanel(doc *document, data preparedPreview, layout layout, projection worldProjector) {
	panel := layout.worldPanel
	doc.f(`<clipPath id="world-clip"><rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="8"/></clipPath>`,
		panel.X+7, panel.Y+58, panel.Width-14, panel.Height-65)
	doc.f(`<g clip-path="url(#world-clip)">`)
	writeGround(doc, projection)
	writeTrajectory(doc, data, projection)
	writeTripod(doc, headpose.Vec3{}, headpose.IdentityQuat(), 0.7, 2.4, "", projection)
	writeTripod(doc, data.scene.DevicePoseInWorld().Translation(),
		data.scene.DevicePoseInWorld().Rotation(), 0.42, 1.7, "d", projection)
	writeTripod(doc, data.scene.CameraPoseInWorld().Translation(),
		data.scene.CameraPoseInWorld().Rotation(), 0.42, 1.7, "c", projection)

	for _, anchor := range data.anchors {
		writeAnchor(doc, anchor, projection)
	}
	writePoseLabels(doc, data, projection)
	doc.f(`</g>`)

	legendY := panel.Y + panel.Height - 18
	doc.f(`<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#55d6ff" stroke-width="2" stroke-linecap="round"/>`,
		panel.X+22, legendY-4, panel.X+48, legendY-4)
	doc.f(`<circle cx="%.1f" cy="%.1f" r="3" fill="#55d6ff"/>`, panel.X+35, legendY-4)
	doc.f(`<text x="%.1f" y="%.1f" fill="#7991b0" font-family="ui-monospace, SFMono-Regular, monospace" font-size="10">camera trajectory · %d samples · arrows show optical -Z</text>`,
		panel.X+56, legendY, len(data.trajectory))
}

func writeGround(doc *document, projection worldProjector) {
	corners := []headpose.Vec3{
		{X: -2.2, Z: -2.2},
		{X: 2.2, Z: -2.2},
		{X: 2.2, Z: 2.2},
		{X: -2.2, Z: 2.2},
	}
	doc.f(`<polygon points="`)
	for _, corner := range corners {
		doc.f(`%s `, pointString(projection.project(corner)))
	}
	doc.f(`" fill="#0d1729" fill-opacity="0.72"/>`)
	for i := -2; i <= 2; i++ {
		line := [2]headpose.Vec3{{X: float64(i), Z: -2.2}, {X: float64(i), Z: 2.2}}
		writeWorldLine(doc, line[0], line[1], projection, "#172842", 0.9)
		line = [2]headpose.Vec3{{X: -2.2, Z: float64(i)}, {X: 2.2, Z: float64(i)}}
		writeWorldLine(doc, line[0], line[1], projection, "#172842", 0.9)
	}
}

func writeWorldLine(
	doc *document,
	from, to headpose.Vec3,
	projection worldProjector,
	color string,
	width float64,
) {
	a, b := projection.project(from), projection.project(to)
	doc.f(`<line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="%s" stroke-width="%.1f"/>`,
		a.X, a.Y, b.X, b.Y, color, width)
}

func writeTrajectory(doc *document, data preparedPreview, projection worldProjector) {
	doc.f(`<polyline points="`)
	for _, sample := range data.trajectory {
		doc.f(`%s `, pointString(projection.project(sample.Translation())))
	}
	doc.f(`" fill="none" stroke="#55d6ff" stroke-width="8" stroke-opacity="0.10" stroke-linejoin="round"/>`)
	doc.f(`<polyline points="`)
	for _, sample := range data.trajectory {
		doc.f(`%s `, pointString(projection.project(sample.Translation())))
	}
	doc.f(`" fill="none" stroke="#55d6ff" stroke-width="2" stroke-linejoin="round"/>`)

	for i, sample := range data.trajectory {
		position := sample.Translation()
		forward := sample.ApplyDirection(headpose.Vec3{Z: -1})
		start := projection.project(position)
		end := projection.project(position.Add(forward.Scale(0.55)))
		writeArrow(doc, start, end, "#55d6ff", 1.1, 0.58)
		doc.f(`<g><circle cx="%.2f" cy="%.2f" r="2.8" fill="#0b1324" stroke="#55d6ff" stroke-width="1.4"/><title>%s</title></g>`,
			start.X, start.Y, xmlEscape(data.labels[i]))
		if i == 0 || i == len(data.trajectory)-1 {
			label := "S"
			if i != 0 {
				label = "E"
			}
			doc.f(`<text x="%.2f" y="%.2f" fill="#bceeff" font-family="ui-monospace, SFMono-Regular, monospace" font-size="11" font-weight="700">%s</text>`,
				start.X+6, start.Y-6, label)
		}
	}
}

func writeTripod(
	doc *document,
	origin headpose.Vec3,
	rotation headpose.Quat,
	length float64,
	width float64,
	prefix string,
	projection worldProjector,
) {
	axes := []struct {
		label string
		axis  headpose.Vec3
		color string
	}{
		{"X", headpose.Vec3{X: 1}, "#ff6b6b"},
		{"Y", headpose.Vec3{Y: 1}, "#7ee787"},
		{"Z", headpose.Vec3{Z: 1}, "#6aa9ff"},
	}
	center := projection.project(origin)
	doc.f(`<circle cx="%.2f" cy="%.2f" r="2.4" fill="#e7f1ff"/>`, center.X, center.Y)
	for _, item := range axes {
		tipWorld := origin.Add(rotation.Rotate(item.axis).Scale(length))
		tip := projection.project(tipWorld)
		writeArrow(doc, center, tip, item.color, width, 1)
		doc.f(`<text x="%.2f" y="%.2f" fill="%s" font-family="ui-monospace, SFMono-Regular, monospace" font-size="9" font-weight="700">%s%s</text>`,
			tip.X+4, tip.Y-4, item.color, prefix, item.label)
	}
}

func writeAnchor(doc *document, anchor Anchor, projection worldProjector) {
	p := projection.project(anchor.Position)
	doc.f(`<circle cx="%.2f" cy="%.2f" r="13" fill="%s" opacity="0.22" filter="url(#soft-glow)"/>`,
		p.X, p.Y, anchor.Color)
	doc.f(`<circle cx="%.2f" cy="%.2f" r="6.5" fill="%s" stroke="#f4f8ff" stroke-width="1.2"/>`,
		p.X, p.Y, anchor.Color)
	doc.f(`<text x="%.2f" y="%.2f" fill="%s" font-family="ui-sans-serif, system-ui, sans-serif" font-size="12" font-weight="650" stroke="#080d18" stroke-width="3.2" paint-order="stroke">%s</text>`,
		p.X+11, p.Y-10, anchor.Color, xmlEscape(anchor.Label))
}

func writePoseLabels(doc *document, data preparedPreview, projection worldProjector) {
	device := projection.project(data.scene.DevicePoseInWorld().Translation())
	camera := projection.project(data.scene.CameraPoseInWorld().Translation())
	doc.f(`<text x="%.2f" y="%.2f" fill="#9db4d2" font-family="ui-sans-serif, system-ui, sans-serif" font-size="10">device pose</text>`,
		device.X-58, device.Y-18)
	doc.f(`<text x="%.2f" y="%.2f" fill="#9db4d2" font-family="ui-sans-serif, system-ui, sans-serif" font-size="10">camera pose</text>`,
		camera.X+10, camera.Y+20)
}
