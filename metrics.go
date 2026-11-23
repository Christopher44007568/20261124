package main

import "fmt"

type Result struct {
	Name         string
	Decisions    []Decision
	TotalSamples float64
	Switches     int
	Boosts       int
	Hits         int
	FalseAlarms  int
	Misses       int
	AvgLead      float64
	AvgPredErr   float64
	RevertMax    int
}

func Run(s Strategy, frames []Frame, saccs []Saccade) *Result {
	s.Init(frames[0].GazeX, frames[0].GazeY, len(frames))
	res := &Result{Name: s.Name(), Decisions: make([]Decision, len(frames))}
	for i, f := range frames {
		var prev Frame
		if i > 0 {
			prev = frames[i-1]
		}
		d := s.Step(i, f, prev)
		res.Decisions[i] = d
		res.TotalSamples += d.Rate
	}
	res.Switches = s.Switches()
	res.RevertMax = s.RevertMax()

	events := s.Events()
	matched := map[int]bool{}
	leadSum, errSum := 0.0, 0.0
	errN := 0
	for _, e := range events {
		if e.End >= len(frames) {
			continue
		}
		res.Boosts++
		best, bestD := -1, 1<<30
		for si, sc := range saccs {
			if matched[si] {
				continue
			}
			if sc.Onset < e.Start-15 || sc.Onset > e.End+15 {
				continue
			}
			d := sc.Onset - e.Start
			if d < 0 {
				d = -d
			}
			if d < bestD {
				bestD = d
				best = si
			}
		}
		if best < 0 {
			res.FalseAlarms++
			continue
		}
		onset := saccs[best].Onset
		if e.PredictedOnset < 0 {
			if onset >= e.Start-10 && onset <= e.End+4 {
				matched[best] = true
				res.Hits++
				leadSum += float64(onset - e.Start)
			}
			continue
		}
		switch {
		case onset >= e.Start-2 && onset <= e.End+8:
			matched[best] = true
			res.Hits++
			leadSum += float64(onset - e.Start)
			d := e.PredictedOnset - onset
			if d < 0 {
				d = -d
			}
			errSum += float64(d)
			errN++
		case onset < e.Start-2:
			matched[best] = true
		default:
			res.FalseAlarms++
		}
	}
	if len(events) > 0 {
		res.Misses = len(saccs) - res.Hits
	}
	if res.Hits > 0 {
		res.AvgLead = leadSum / float64(res.Hits)
	}
	if errN > 0 {
		res.AvgPredErr = errSum / float64(errN)
	}
	return res
}

func printTable(results []*Result) {
	fmt.Printf("%-10s %8s %8s %6s %5s %4s %5s %8s %8s %8s\n",
		"strategy", "samples", "switches", "boosts", "hits", "FA", "miss", "lead(f)", "err(f)", "revert(f)")
	for _, r := range results {
		fmt.Printf("%-10s %8.1f %8d %6d %5d %4d %5d %8.1f %8.1f %8d\n",
			r.Name, r.TotalSamples, r.Switches, r.Boosts, r.Hits,
			r.FalseAlarms, r.Misses, r.AvgLead, r.AvgPredErr, r.RevertMax)
	}
}
