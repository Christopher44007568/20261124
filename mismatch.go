package posetimeline

import "sort"

// markEyeMismatches pairs left/right samples of one batch by their rank
// within each eye (after clock translation and timestamp sorting) and flags
// both sides when their timestamps differ by more than EyeTolerance.
func (v *Validator) markEyeMismatches(list []queued) {
	var left, right []int
	for i := range list {
		if !list[i].known {
			continue
		}
		switch list[i].sample.Eye {
		case EyeLeft:
			left = append(left, i)
		case EyeRight:
			right = append(right, i)
		}
	}
	sort.SliceStable(left, func(a, b int) bool {
		return list[left[a]].global < list[left[b]].global
	})
	sort.SliceStable(right, func(a, b int) bool {
		return list[right[a]].global < list[right[b]].global
	})

	pairs := len(left)
	if len(right) < pairs {
		pairs = len(right)
	}
	for i := 0; i < pairs; i++ {
		li, ri := left[i], right[i]
		diff := list[li].global - list[ri].global
		if diff < 0 {
			diff = -diff
		}
		if diff > v.cfg.EyeTolerance {
			list[li].reasons |= RejEyeMismatch
			list[ri].reasons |= RejEyeMismatch
		}
	}
}
