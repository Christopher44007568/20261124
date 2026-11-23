package main

// Predictor estimates the next saccade onset from the running mean of
// observed fixation durations. It samples Low during confident fixation,
// High inside a conservative window around the predicted onset. On a
// false alarm it falls back to fixed Base sampling within one frame
// (well inside the revert deadline) and stays there for fallbackLen
// frames. A global sample budget caps total spend.
type Predictor struct {
	tr          *CenterTracker
	lead        int
	tol         int
	fallbackLen int
	budgetMult  float64
	budget      float64
	spent       float64
	exhausted   bool
	lastOffset  int
	fixSum      int
	fixN        int
	armed       bool
	predOnset   int
	boostStart  int
	boostEnd    int
	inSacc      bool
	fallbackTo  int
	revertMax   int
	events      []BoostEvent
	det         saccadeDetector
}

func NewPredictor(lead, tol, fallbackLen int, budgetMult float64) *Predictor {
	return &Predictor{lead: lead, tol: tol, fallbackLen: fallbackLen,
		budgetMult: budgetMult, lastOffset: -1, fallbackTo: -1}
}

func (p *Predictor) Name() string { return "predict" }

func (p *Predictor) Init(x, y float64, total int) {
	p.tr = NewCenterTracker(x, y)
	p.budget = p.budgetMult * RateBase * float64(total)
}

func (p *Predictor) Step(i int, f, prev Frame) Decision {
	p.tr.Update(f.ObsX, f.ObsY, f.ObsValid)
	onset, offset := p.det.step(f)
	if onset {
		if p.lastOffset >= 0 {
			p.fixSum += i - 2 - p.lastOffset
			p.fixN++
		}
		p.armed = false
	}
	if offset {
		p.lastOffset = i
		if p.fixN >= 2 && !p.exhausted {
			avg := p.fixSum / p.fixN
			p.predOnset = i + avg
			p.boostStart = p.predOnset - p.lead
			p.boostEnd = p.predOnset + p.tol
			p.armed = true
			p.events = append(p.events, BoostEvent{Start: p.boostStart, End: p.boostEnd,
				PredictedOnset: p.predOnset, MatchedOnset: -1})
		}
	}

	if p.armed && i > p.boostEnd {
		p.armed = false
		p.fallbackTo = i + p.fallbackLen
		if p.revertMax < 1 {
			p.revertMax = 1
		}
	}

	rate := RateBase
	boost := false
	fallback := false
	switch {
	case p.exhausted || i < p.fallbackTo:
		rate = RateBase
		fallback = true
	case p.armed && i >= p.boostStart && i <= p.boostEnd:
		rate = RateHigh
		boost = true
	case p.armed && i < p.boostStart:
		rate = RateLow
	}
	p.spent += rate
	if p.spent > p.budget && !p.exhausted {
		p.exhausted = true
		p.armed = false
	}
	return Decision{Rate: rate, CenterX: p.tr.CX, CenterY: p.tr.CY, Boost: boost, Fallback: fallback}
}

func (p *Predictor) Events() []BoostEvent { return p.events }
func (p *Predictor) Switches() int        { return p.tr.Switches }
func (p *Predictor) RevertMax() int       { return p.revertMax }
