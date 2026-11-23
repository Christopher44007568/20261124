package foveal

// processPixel reprojects the previous history, applies the invalidation
// ranges and neighborhood clamping, then blends with the current sample using
// the zone-specific history length.
func (d *Denoiser) processPixel(f *Frame, idx, x, y int, extremeExposure bool, stats *Stats) {
	cfg := d.params
	zone := d.zoneMask[idx]
	var current [3]float32
	copy(current[:], f.RGB[idx*3:idx*3+3])

	if zone == ZoneFovea {
		stats.FovealPixels++
	} else {
		stats.PeripheralPixels++
	}

	var history [3]float32
	var age float32
	valid := false
	reason := d.reset[idx]

	src := d.reproject(f, x, y, zone)
	if !src.inside {
		// Either the reprojection truly left the frame, or every bilinear tap
		// belonged to the other zone. The latter is a ownership change: start
		// fresh rather than smearing peripheral history into the fovea.
		mv := f.MV[y*f.Width+x]
		sx := float32(x) - mv.X
		sy := float32(y) - mv.Y
		if sx < 0 || sx > float32(f.Width-1) || sy < 0 || sy > float32(f.Height-1) {
			reason |= ReasonBorder
		} else {
			reason |= ReasonZone
		}
	} else if f.Depth != nil && d.depthDisoccludes(f, idx, src.dominant) {
		reason |= ReasonDisocclusion
	} else {
		history = src.color
		age = src.age
		valid = true

		// Moderate exposure changes are compensated so static regions keep
		// their converged history; extreme jumps already flagged a full reset.
		if f.ExposureScale > 0 && f.ExposureScale != 1 && !extremeExposure {
			for c := 0; c < 3; c++ {
				history[c] *= f.ExposureScale
			}
		}

		history = d.neighborhoodClamp(f, src.centerX, src.centerY, zone, history, cfg.ClampExpand)

		if highlightActive(current, history, &cfg) {
			reason |= ReasonHighlight
			valid = false
		}
	}

	if reason != ReasonNone {
		valid = false
	}

	historyLen := float32(cfg.FovealHistory)
	if zone == ZonePeriphery {
		historyLen = float32(cfg.PeripheralHistory)
	}

	st := &d.state[idx]
	st.zone = zone
	if !valid {
		st.hist = current
		st.age = 1
		st.initialized = true
	} else {
		// EMA that converges to the zone's equivalent sample count: alpha
		// shrinks while ramping up, then stays at 1/historyLen.
		newAge := min(age+1, historyLen)
		alpha := 1 / newAge
		for c := 0; c < 3; c++ {
			st.hist[c] = history[c] + alpha*(current[c]-history[c])
		}
		st.age = newAge
	}

	copy(d.out[idx*3:idx*3+3], st.hist[:])
	d.reset[idx] = reason
	d.collectStats(zone, reason, st.age, stats)
}

func (d *Denoiser) collectStats(zone Zone, reason ResetReason, age float32, stats *Stats) {
	if reason&ReasonEye != 0 {
		stats.ResetEye++
	}
	if reason&ReasonMotion != 0 {
		stats.ResetMotion++
	}
	if reason&ReasonDisocclusion != 0 {
		stats.ResetDisocclusion++
	}
	if reason&ReasonExposure != 0 {
		stats.ResetExposure++
	}
	if reason&ReasonHighlight != 0 {
		stats.ResetHighlight++
	}
	if reason&ReasonBorder != 0 {
		stats.ResetBorder++
	}
	if reason&ReasonZone != 0 {
		stats.ResetZone++
	}
	if zone == ZoneFovea {
		stats.MeanFovealAge += float64(age)
	} else {
		stats.MeanPeriphAge += float64(age)
	}
}

type reprojected struct {
	dominant         int
	centerX, centerY int
	color            [3]float32
	age              float32
	inside           bool
}

// reproject resolves the previous-frame position of the content now at (x,y)
// with zone-constrained bilinear taps. Taps belonging to another zone, or to
// uninitialized state, get zero weight; surviving weights are renormalized so
// peripheral accumulation can never leak into the fovea.
func (d *Denoiser) reproject(f *Frame, x, y int, zone Zone) reprojected {
	w, h := f.Width, f.Height
	mv := f.MV[y*w+x]
	sx := float32(x) - mv.X
	sy := float32(y) - mv.Y
	if sx < 0 || sx > float32(w-1) || sy < 0 || sy > float32(h-1) {
		return reprojected{inside: false}
	}

	x0 := int(sx)
	y0 := int(sy)
	x1 := min(x0+1, w-1)
	y1 := min(y0+1, h-1)
	fx := sx - float32(x0)
	fy := sy - float32(y0)

	type tap struct {
		idx    int
		weight float32
	}
	taps := [4]tap{
		{y0*w + x0, (1 - fx) * (1 - fy)},
		{y0*w + x1, fx * (1 - fy)},
		{y1*w + x0, (1 - fx) * fy},
		{y1*w + x1, fx * fy},
	}

	var sumW, domW float32
	var out reprojected
	for _, t := range taps {
		st := &d.state[t.idx]
		if !st.initialized || st.zone != zone {
			continue
		}
		sumW += t.weight
		for c := 0; c < 3; c++ {
			out.color[c] += st.hist[c] * t.weight
		}
		out.age += st.age * t.weight
		if t.weight > domW {
			domW = t.weight
			out.dominant = t.idx
			out.centerX = t.idx % w
			out.centerY = t.idx / w
		}
	}
	if sumW <= 0 {
		return reprojected{inside: false}
	}
	inv := 1 / sumW
	for c := 0; c < 3; c++ {
		out.color[c] *= inv
	}
	out.age *= inv
	out.inside = true
	return out
}

// depthDisoccludes reports whether the reprojected background is much farther
// than the current surface, indicating newly revealed content.
func (d *Denoiser) depthDisoccludes(f *Frame, cur, src int) bool {
	curD := f.Depth[cur]
	srcD := f.Depth[src]
	if curD <= 0 {
		return false
	}
	return (srcD-curD)/curD > d.params.DepthDisocclusionThreshold
}

// neighborhoodClamp constrains reprojected history to the 3x3 min/max envelope
// of the current frame around (cx,cy), expanded by expand. Only taps in the
// same zone participate, keeping fovea and periphery histories separated.
func (d *Denoiser) neighborhoodClamp(f *Frame, cx, cy int, zone Zone, hist [3]float32, expand float32) [3]float32 {
	w, h := f.Width, f.Height
	var lo, hi [3]float32
	have := false
	for yy := max(cy-1, 0); yy <= min(cy+1, h-1); yy++ {
		for xx := max(cx-1, 0); xx <= min(cx+1, w-1); xx++ {
			pp := yy*w + xx
			if d.zoneMask[pp] != zone {
				continue
			}
			for c := 0; c < 3; c++ {
				v := f.RGB[pp*3+c]
				if !have {
					lo[c], hi[c] = v, v
				} else {
					lo[c] = min(lo[c], v)
					hi[c] = max(hi[c], v)
				}
			}
			have = true
		}
	}
	if !have {
		return hist
	}
	for c := 0; c < 3; c++ {
		span := hi[c] - lo[c]
		lo[c] -= span * expand
		hi[c] += span * expand
		hist[c] = max(lo[c], min(hi[c], hist[c]))
	}
	return hist
}
