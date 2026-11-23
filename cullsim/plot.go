package main

import (
	"fmt"
	"math"
)

var stratColors = map[string]RGB{
	"none":    {95, 145, 250},
	"union":   {240, 90, 90},
	"predict": {90, 210, 130},
}

// DrawFrame renders a top-down annotated view of one frame for one
// strategy. Missed objects (holes) are drawn in bright magenta with a
// ring: a hole is never painted as background.
func DrawFrame(env *FrameEnv, candSet, req map[int]bool, name string,
	frame int, st FrameStat, path string) error {
	const size = 920
	c := NewCanvas(size, size, colBg)
	cam := Cam{Scale: 1.42, Cx: size / 2, Cy: size / 2}
	for _, eye := range []Vec2{env.EyeL, env.EyeR} {
		ex, ey := cam.P(eye)
		d1 := eye.Add(FromAngle(env.Pose.Yaw - env.P.FovHalf).Mul(env.P.MaxDist))
		d2 := eye.Add(FromAngle(env.Pose.Yaw + env.P.FovHalf).Mul(env.P.MaxDist))
		x1, y1 := cam.P(d1)
		x2, y2 := cam.P(d2)
		c.FillTri([2]int{ex, ey}, [2]int{x1, y1}, [2]int{x2, y2}, colEye, 0.10)
	}
	for _, w := range env.Scene.Walls {
		x0, y0 := cam.P(w.A)
		x1, y1 := cam.P(w.B)
		c.Line(x0, y0, x1, y1, colWall)
	}
	lx, ly := cam.P(env.Light)
	c.Ring(lx, ly, int(env.P.LightRadius*cam.Scale), RGB{110, 100, 55})
	c.FillCircle(lx, ly, 6, colLight)
	for i, o := range env.Scene.Objects {
		x, y := cam.P(o.Pos)
		r := int(math.Max(2, o.Radius*cam.Scale))
		inCand, inReq := candSet[i], req[i]
		switch {
		case inReq && !inCand:
			c.FillCircle(x, y, r+2, colMiss)
			c.Ring(x, y, r+6, colMiss)
			c.Line(x-r-6, y, x+r+6, y, colMiss)
			c.Line(x, y-r-6, x, y+r+6, colMiss)
		case inCand && inReq:
			c.FillCircle(x, y, r+1, colRendered)
		case inCand && !inReq:
			c.FillCircle(x, y, r+1, colOverdraw)
		default:
			c.FillCircle(x, y, r, colObjIdle)
		}
	}
	vx, vy := cam.P(env.Pose.Pos)
	c.FillCircle(vx, vy, 5, colViewer)
	for _, eye := range []Vec2{env.EyeL, env.EyeR} {
		ex, ey := cam.P(eye)
		c.FillCircle(ex, ey, 3, colEye)
	}
	c.Text(14, 12, fmt.Sprintf("FRAME %02d/%d STRATEGY %s", frame, totalFrames,
		upcase(name)), colText, 2)
	c.Text(14, 32, fmt.Sprintf("CAND %d RENDER %d MISS %d OVER %d",
		st.Candidates, st.Rendered, st.Misses, st.Overdraw), colText, 2)
	legend := []struct {
		col RGB
		txt string
	}{
		{colRendered, "RENDERED"},
		{colOverdraw, "OVERDRAW"},
		{colMiss, "MISS HOLE NOT BG"},
		{colObjIdle, "IDLE"},
		{colLight, "LIGHT"},
	}
	lxPos := 14
	for _, l := range legend {
		c.FillCircle(lxPos+5, size-16, 5, l.col)
		c.Text(lxPos+14, size-20, l.txt, colText, 1)
		lxPos += 14 + TextWidth(l.txt, 1) + 16
	}
	return c.SavePNG(path)
}

func upcase(s string) string {
	b := []byte(s)
	for i, ch := range b {
		if ch >= 'a' && ch <= 'z' {
			b[i] = ch - 32
		}
	}
	return string(b)
}

// DrawChart plots candidates / misses / overdraw over time for every
// strategy into a single PNG with three stacked panels.
func DrawChart(stats map[string][]FrameStat, order []string, path string) error {
	const (
		W        = 960
		H        = 760
		marginL  = 60
		marginR  = 20
		panelH   = 200
		panelGap = 40
		top      = 88
	)
	c := NewCanvas(W, H, colBg)
	c.Text(marginL, 16, "CULLING STRATEGY COMPARISON (90 FRAMES)", colText, 2)

	// Legend.
	lx := marginL
	for _, name := range order {
		col := stratColors[name]
		for dy := 0; dy < 4; dy++ {
			c.Line(lx, 44+dy, lx+18, 44+dy, col)
		}
		c.Text(lx+24, 42, upcase(name), colText, 1)
		lx += 24 + TextWidth(upcase(name), 1) + 30
	}

	panels := []struct {
		title string
		get   func(FrameStat) int
	}{
		{"CANDIDATES", func(s FrameStat) int { return s.Candidates }},
		{"MISSES (HOLES)", func(s FrameStat) int { return s.Misses }},
		{"OVERDRAW", func(s FrameStat) int { return s.Overdraw }},
	}

	for pi, p := range panels {
		y0 := top + pi*(panelH+panelGap)
		y1 := y0 + panelH
		maxV := 1
		for _, name := range order {
			for _, s := range stats[name] {
				if v := p.get(s); v > maxV {
					maxV = v
				}
			}
		}
		// Gridlines and axis labels.
		grid := RGB{46, 50, 60}
		for g := 0; g <= 4; g++ {
			gy := y1 - g*panelH/4
			c.Line(marginL, gy, W-marginR, gy, grid)
			c.Text(6, gy-3, fmt.Sprintf("%d", maxV*g/4), colText, 1)
		}
		c.Text(marginL, y0-14, p.title, colText, 1)
		// Frame ticks.
		for f := 0; f <= totalFrames; f += 30 {
			tx := marginL + f*(W-marginL-marginR)/totalFrames
			c.Line(tx, y1, tx, y1+4, colText)
			c.Text(tx-6, y1+8, fmt.Sprintf("%d", f), colText, 1)
		}
		// Polylines.
		for _, name := range order {
			col := stratColors[name]
			ss := stats[name]
			px, py := 0, 0
			for i, s := range ss {
				x := marginL + s.Frame*(W-marginL-marginR)/totalFrames
				y := y1 - p.get(s)*panelH/maxV
				if i > 0 {
					c.Line(px, py, x, y, col)
				}
				px, py = x, y
			}
		}
	}
	return c.SavePNG(path)
}
