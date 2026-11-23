package main

import (
	"math"
	"math/rand"
)

const obsNoise = 2.0

type Frame struct {
	GazeX, GazeY float64
	ObsX, ObsY   float64
	ObsValid     bool
	Blink        bool
	Saccade      bool
}

type SimConfig struct {
	Frames    int
	Width     int
	Height    int
	NumSacc   int
	NumBlink  int
	Seed      int64
	AnomalyAt int
}

type Saccade struct {
	Onset, Dur int
}

func Simulate(cfg SimConfig) ([]Frame, []Saccade) {
	rng := rand.New(rand.NewSource(cfg.Seed))
	frames := make([]Frame, cfg.Frames)

	var saccs []Saccade
	t := 40
	for len(saccs) < cfg.NumSacc {
		gap := 85 + rng.Intn(30)
		onset := t + gap
		amp := 80 + rng.Float64()*220
		dur := int(math.Round(2 + amp/60.0))
		if dur < 2 {
			dur = 2
		}
		if dur > 8 {
			dur = 8
		}
		if onset+dur >= cfg.Frames-20 {
			break
		}
		saccs = append(saccs, Saccade{Onset: onset, Dur: dur})
		t = onset + dur
	}

	fx, fy := float64(cfg.Width)/2, float64(cfg.Height)/2
	dx, dy := 0.0, 0.0
	si := 0
	var sx, sy, tx, ty float64
	for i := 0; i < cfg.Frames; i++ {
		if si < len(saccs) && i >= saccs[si].Onset && i < saccs[si].Onset+saccs[si].Dur {
			frames[i].Saccade = true
			if i == saccs[si].Onset {
				sx, sy = fx+dx, fy+dy
				for {
					tx = float64(cfg.Width) * (0.15 + 0.7*rng.Float64())
					ty = float64(cfg.Height) * (0.15 + 0.7*rng.Float64())
					if math.Hypot(tx-sx, ty-sy) > 80 {
						break
					}
				}
			}
			p := float64(i-saccs[si].Onset+1) / float64(saccs[si].Dur)
			s := p * p * (3 - 2*p)
			frames[i].GazeX = sx + (tx-sx)*s
			frames[i].GazeY = sy + (ty-sy)*s
			if i == saccs[si].Onset+saccs[si].Dur-1 {
				fx, fy = tx, ty
				dx, dy = 0, 0
				si++
			}
		} else {
			dx += rng.NormFloat64()*0.6 - 0.02*dx
			dy += rng.NormFloat64()*0.6 - 0.02*dy
			frames[i].GazeX = fx + dx
			frames[i].GazeY = fy + dy
		}
	}

	blinks := 0
	for blinks < cfg.NumBlink {
		start := 60 + rng.Intn(cfg.Frames-140)
		ok := true
		for j := start - 6; j < start+12 && ok; j++ {
			if j >= 0 && j < cfg.Frames && (frames[j].Saccade || frames[j].Blink) {
				ok = false
			}
		}
		if !ok {
			continue
		}
		for j := start; j < start+6; j++ {
			frames[j].Blink = true
		}
		blinks++
	}

	for i := range frames {
		if frames[i].Blink {
			frames[i].ObsValid = false
			continue
		}
		frames[i].ObsValid = true
		frames[i].ObsX = frames[i].GazeX + rng.NormFloat64()*obsNoise
		frames[i].ObsY = frames[i].GazeY + rng.NormFloat64()*obsNoise
	}
	if cfg.AnomalyAt > 0 && cfg.AnomalyAt < cfg.Frames {
		frames[cfg.AnomalyAt].ObsX += 150
		frames[cfg.AnomalyAt].ObsY -= 120
	}
	return frames, saccs
}
