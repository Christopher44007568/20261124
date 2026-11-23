package compositor

import (
	"fmt"
	"math"
)

type selectedSurface struct {
	id       string
	depth    float64
	hasDepth bool
}

func truthSurface(scene Scene, eye Eye, x, y int) selectedSurface {
	best := selectedSurface{}
	bestDepth := math.Inf(1)
	bestOrder := math.MinInt
	fallback := selectedSurface{}
	fallbackOrder := math.MinInt

	for _, layer := range scene.Layers {
		i := y*scene.Width + x
		if i < 0 || i >= len(layer.Coverage) || !layer.Coverage[i] {
			continue
		}
		if layer.Order > fallbackOrder {
			fallbackOrder = layer.Order
			fallback = selectedSurface{id: layer.ID}
		}
		depths := layer.Depths(eye)
		if i >= len(depths) {
			continue
		}
		depth := depths[i]
		if !validDepth(depth) {
			continue
		}
		if depth < bestDepth-1e-12 ||
			(math.Abs(depth-bestDepth) <= 1e-12 && layer.Order > bestOrder) {
			bestDepth = depth
			bestOrder = layer.Order
			best = selectedSurface{id: layer.ID, depth: depth, hasDepth: true}
		}
	}
	if best.id != "" {
		return best
	}
	return fallback
}

func maxOrderSurface(candidates []int, layers []Layer, systemOnly bool) int {
	best := -1
	bestOrder := math.MinInt
	for _, i := range candidates {
		layer := layers[i]
		if systemOnly && !layer.System {
			continue
		}
		if layer.Order > bestOrder {
			best = i
			bestOrder = layer.Order
		}
	}
	return best
}

func nearestSurface(candidates []int, decisions []CandidateDecision) (int, bool) {
	best := -1
	bestDepth := math.Inf(1)
	bestOrder := math.MinInt
	for _, i := range candidates {
		candidate := decisions[i]
		if !candidate.DepthValid {
			continue
		}
		if candidate.Depth < bestDepth-1e-12 ||
			(math.Abs(candidate.Depth-bestDepth) <= 1e-12 && candidate.Order > bestOrder) {
			best = i
			bestDepth = candidate.Depth
			bestOrder = candidate.Order
		}
	}
	return best, best >= 0
}

func hasQuantumOverlap(best int, candidates []int, decisions []CandidateDecision, step float64) bool {
	if best < 0 || step <= 0 {
		return false
	}
	chosen := decisions[best]
	for _, i := range candidates {
		if i == best || !decisions[i].DepthValid {
			continue
		}
		if quantizedTie(chosen.Depth, decisions[i].Depth, step) {
			return true
		}
	}
	return false
}

func decidePixel(scene Scene, submission Submission, policy Policy, eye Eye, x, y int) (PixelDecision, int) {
	decision := PixelDecision{X: x, Y: y, Eye: eye}
	covered := make([]int, 0, len(submission.Layers))
	candidates := make([]CandidateDecision, len(submission.Layers))
	invalidSamples := 0

	for i, layer := range submission.Layers {
		pi := y*scene.Width + x
		if pi < 0 || pi >= len(layer.Coverage) || !layer.Coverage[pi] {
			continue
		}
		covered = append(covered, i)
		depths := layer.Depths(eye)
		if policy.LeftDepthForBoth {
			depths = layer.Left
		}
		candidate := CandidateDecision{
			LayerID:   layer.ID,
			Kind:      layer.Kind,
			System:    layer.System,
			Order:     layer.Order,
			SourceEye: eye,
			Status:    StatusOccluded,
		}
		if policy.LeftDepthForBoth {
			candidate.SourceEye = LeftEye
		}
		if pi >= len(depths) {
			candidate.Status = StatusMissing
			invalidSamples++
		} else {
			depth := depths[pi]
			if validDepth(depth) {
				candidate.Depth = depth
				candidate.DepthValid = true
			} else {
				candidate.Status = StatusInvalid
				invalidSamples++
			}
		}
		candidates[i] = candidate
	}

	if len(covered) == 0 {
		decision.Reason = ReasonBackground
		return decision, 0
	}

	valid := make([]int, 0, len(covered))
	for _, i := range covered {
		if candidates[i].DepthValid {
			valid = append(valid, i)
		}
	}

	winner := -1
	switch {
	case policy.SystemAlwaysFront && maxOrderSurface(covered, submission.Layers, true) >= 0:
		winner = maxOrderSurface(covered, submission.Layers, true)
		decision.Reason = ReasonSystemFront
		decision.Ambiguous = true
	case submission.Mode == DepthNone:
		winner = maxOrderSurface(covered, submission.Layers, false)
		decision.Reason = ReasonNoDepth
	case policy.IgnoreIncomplete && len(valid) > 0:
		winner, _ = nearestSurface(valid, candidates)
		decision.Reason = ReasonNearestDepth
		decision.Ambiguous = len(valid) != len(covered)
	case len(valid) == 0:
		winner = maxOrderSurface(covered, submission.Layers, false)
		decision.Reason = ReasonNoValidDepth
		decision.Ambiguous = true
	case len(valid) != len(covered):
		winner = maxOrderSurface(covered, submission.Layers, false)
		decision.Reason = ReasonIncompleteDepth
		decision.Ambiguous = true
	default:
		winner, _ = nearestSurface(valid, candidates)
		decision.Reason = ReasonNearestDepth
		if hasQuantumOverlap(winner, valid, candidates, submission.Quantum) {
			decision.Reason = ReasonQuantizationTie
			decision.Ambiguous = true
		}
	}

	if winner >= 0 {
		decision.Winner = submission.Layers[winner].ID
		candidates[winner].Selected = true
		if candidates[winner].DepthValid {
			candidates[winner].Status = StatusSelected
		}
	}
	for _, i := range covered {
		decision.Candidates = append(decision.Candidates, candidates[i])
	}

	return decision, invalidSamples
}

// Composite evaluates one submission independently for each eye. It never
// reads the other eye's depth array.
func Composite(scene Scene, submission Submission, policy Policy) Result {
	result := Result{
		Submission: submission.Name,
		Mode:       submission.Mode,
		Policy:     policy.Name,
		Width:      scene.Width,
		Height:     scene.Height,
		layers:     submission.Layers,
	}
	if result.Policy == "" {
		result.Policy = SafePolicy.Name
	}
	result.Eyes = make([]EyeResult, 0, 2)

	for _, eye := range []Eye{LeftEye, RightEye} {
		eyeResult := EyeResult{
			Eye:       eye,
			Decisions: make([]PixelDecision, 0, scene.Pixels()),
		}
		for y := 0; y < scene.Height; y++ {
			for x := 0; x < scene.Width; x++ {
				decision, invalidSamples := decidePixel(scene, submission, policy, eye, x, y)
				eyeResult.Decisions = append(eyeResult.Decisions, decision)
				eyeResult.Summary.Pixels++
				if decision.Winner != "" {
					eyeResult.Summary.CoveredPixels++
				}
				if decision.Reason == ReasonNoDepth ||
					decision.Reason == ReasonIncompleteDepth ||
					decision.Reason == ReasonNoValidDepth {
					eyeResult.Summary.OrderFallbacks++
				}
				if submission.Mode != DepthNone {
					eyeResult.Summary.InvalidDepthSamples += invalidSamples
					if invalidSamples > 0 {
						eyeResult.Summary.IncompleteDepthPixels++
					}
				}
				if decision.Reason == ReasonQuantizationTie {
					eyeResult.Summary.QuantizationAmbiguous++
				}

				expected := truthSurface(scene, eye, x, y)
				if decision.Winner == expected.id {
					eyeResult.Summary.CorrectPixels++
					continue
				}
				err := OcclusionError{
					X: x, Y: y, Eye: eye,
					Expected: expected.id,
					Actual:   decision.Winner,
				}
				switch {
				case expected.id != "" && decision.Winner == "":
					err.Kind = ErrorMissingSurface
				case expected.id == "":
					err.Kind = ErrorUnexpectedLayer
				default:
					err.Kind = ErrorWrongFront
				}
				err.Detail = fmt.Sprintf("expected %q, selected %q", expected.id, decision.Winner)
				eyeResult.Errors = append(eyeResult.Errors, err)
				eyeResult.Summary.WrongOcclusions++
			}
		}
		result.Eyes = append(result.Eyes, eyeResult)
		addSummary(&result.Summary, eyeResult.Summary)
	}
	return result
}

func addSummary(dst *Summary, src Summary) {
	dst.Pixels += src.Pixels
	dst.CoveredPixels += src.CoveredPixels
	dst.CorrectPixels += src.CorrectPixels
	dst.WrongOcclusions += src.WrongOcclusions
	dst.OrderFallbacks += src.OrderFallbacks
	dst.IncompleteDepthPixels += src.IncompleteDepthPixels
	dst.InvalidDepthSamples += src.InvalidDepthSamples
	dst.QuantizationAmbiguous += src.QuantizationAmbiguous
}
