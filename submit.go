package posetimeline

import "sort"

type queued struct {
	index   int
	sample  Sample
	global  int64
	known   bool
	reasons RejectReason
}

// Submit validates one batch against referenceTime (a primary-domain tick)
// and returns a Decision per sample in batch order. Acceptance never depends
// on array position: samples are first translated to the primary clock and
// then processed in timestamp order.
//
// A backwards clock invalidates the eye's timeline. Subsequent samples are
// refused with RejTimelineBroken and no motion is accumulated until Reset
// re-anchors it. Samples from unregistered clocks are individually rejected
// with RejUnknownClock without poisoning the orderable timeline.
func (v *Validator) Submit(referenceTime int64, batch []Sample) []Decision {
	queuedList := make([]queued, len(batch))
	for i, sample := range batch {
		q := queued{index: i, sample: sample}
		if int(sample.Eye) >= len(v.eyes) {
			q.reasons = RejUnknownClock
			queuedList[i] = q
			continue
		}
		global, ok := v.clocks.Translate(sample.Clock, sample.Timestamp)
		if !ok {
			// An untranslatable sample is dropped, but it cannot be placed on
			// the timeline at all, so it does not invalidate the samples that
			// can still be ordered.
			q.reasons = RejUnknownClock
		} else {
			q.known = true
			q.global = global
		}
		queuedList[i] = q
	}

	v.markEyeMismatches(queuedList)

	processing := make([]int, 0, len(queuedList))
	for i := range queuedList {
		if queuedList[i].known {
			processing = append(processing, i)
		}
	}
	sort.SliceStable(processing, func(a, b int) bool {
		qa, qb := queuedList[processing[a]], queuedList[processing[b]]
		if qa.global != qb.global {
			return qa.global < qb.global
		}
		return qa.index < qb.index
	})

	for _, qi := range processing {
		q := &queuedList[qi]
		state := v.eyes[q.sample.Eye]
		reasons := q.reasons // pre-computed bits, e.g. RejEyeMismatch
		// Equality with the last accepted timestamp is a duplicate; strict
		// inequality below is a backwards clock.
		if state.haveLast && q.global == state.lastTime {
			reasons |= RejDuplicateTimestamp
		}
		if q.sample.FrameID != 0 && state.haveFrame && state.lastFrameID == q.sample.FrameID {
			reasons |= RejDuplicateFrame
		}
		if state.haveLast && q.global < state.lastTime {
			reasons |= RejClockBackwards
			state.broken = true
		}
		if q.global > referenceTime {
			reasons |= RejFuture
		} else if v.cfg.MaxPoseAge > 0 && referenceTime-q.global > v.cfg.MaxPoseAge {
			reasons |= RejExpired
		}
		// A break (pre-existing or raised above by a backwards tick) refuses
		// the sample without hiding any of its other rejection reasons.
		if state.broken {
			reasons |= RejTimelineBroken
		}

		if reasons != RejNone {
			q.reasons |= reasons
			continue
		}
		v.accept(state, q)
	}

	decisions := make([]Decision, len(batch))
	for i := range batch {
		q := queuedList[i]
		decisions[i] = Decision{
			Index:         i,
			Sample:        q.sample,
			Accepted:      q.reasons == RejNone,
			Reasons:       q.reasons,
			GlobalTime:    q.global,
			ReferenceTime: referenceTime,
		}
		v.decisions = append(v.decisions, decisions[i])
	}
	return decisions
}

func (v *Validator) accept(state *eyeState, q *queued) {
	state.lastTime = q.global
	state.haveLast = true
	if q.sample.FrameID != 0 {
		state.lastFrameID = q.sample.FrameID
		state.haveFrame = true
	}
	state.lastPose = q.sample.Pose
	state.havePose = true
	state.accepted = append(state.accepted, acceptedEntry{
		globalTime: q.global,
		sample:     q.sample,
	})
	state.integrated++
}
