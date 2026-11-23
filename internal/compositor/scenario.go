package compositor

// DemoScene defines a scene where a near hand overlaps a middle panel and a
// distant system notification. Painter order is correct for this scene, but it
// is only a fallback when per-eye depth cannot decide.
func DemoScene() Scene {
	const width, height = 12, 8
	scene := Scene{Name: "hand-panel-notification", Width: width, Height: height}

	notificationCoverage := rectMask(width, height, 0, 0, width-1, height-1)
	notification := Layer{
		ID:       "notice",
		Kind:     SurfaceNotification,
		System:   true,
		Order:    0,
		Coverage: notificationCoverage,
	}
	notification.Left = depthField(width, height, notificationCoverage, 7.0, 0.002, 0.001)
	notification.Right = depthField(width, height, notificationCoverage, 6.99, 0.0021, 0.001)

	panelCoverage := rectMask(width, height, 2, 1, 9, 6)
	panel := Layer{
		ID:       "panel",
		Kind:     SurfacePanel,
		Order:    1,
		Coverage: panelCoverage,
	}
	panel.Left = depthField(width, height, panelCoverage, 1.20, 0.003, 0.001)
	panel.Right = depthField(width, height, panelCoverage, 1.21, 0.0031, 0.001)

	handCoverage := rectMask(width, height, 6, 2, 11, 7)
	hand := Layer{
		ID:       "hand",
		Kind:     SurfaceHand,
		Order:    2,
		Coverage: handCoverage,
	}
	hand.Left = depthField(width, height, handCoverage, 0.31, 0.002, 0.001)
	hand.Right = depthField(width, height, handCoverage, 0.308, 0.0021, 0.001)

	scene.Layers = []Layer{notification, panel, hand}
	return scene
}

func emptyDepthSubmission(scene Scene, name string) Submission {
	sub := Submission{Name: name, Mode: DepthNone}
	for _, layer := range scene.Layers {
		copy := cloneLayer(layer)
		copy.Left = nil
		copy.Right = nil
		sub.Layers = append(sub.Layers, copy)
	}
	return sub
}

func sparseDepthSubmission(scene Scene, name string, step float64) Submission {
	sub := Submission{Name: name, Mode: DepthSparse, Quantum: step}
	for _, layer := range scene.Layers {
		copy := cloneLayer(layer)
		for y := 0; y < scene.Height; y++ {
			for x := 0; x < scene.Width; x++ {
				i := y*scene.Width + x
				keep := false
				switch layer.Kind {
				case SurfaceHand:
					keep = (x+y)%4 == 0
				case SurfacePanel:
					keep = (x+2*y)%3 == 0
				default:
					keep = (2*x+y)%5 == 0
				}
				if layer.Coverage[i] && keep {
					copy.Left[i] = QuantizeDepth(copy.Left[i], step)
					copy.Right[i] = QuantizeDepth(copy.Right[i], step)
				} else {
					copy.Left[i] = missingDepth()
					copy.Right[i] = missingDepth()
				}
			}
		}
		sub.Layers = append(sub.Layers, copy)
	}
	return sub
}

func fullDepthSubmission(scene Scene, name string, step float64) Submission {
	sub := Submission{Name: name, Mode: DepthFull, Quantum: step}
	for _, layer := range scene.Layers {
		copy := cloneLayer(layer)
		for i := range copy.Coverage {
			if !copy.Coverage[i] {
				continue
			}
			copy.Left[i] = QuantizeDepth(copy.Left[i], step)
			copy.Right[i] = QuantizeDepth(copy.Right[i], step)
		}
		sub.Layers = append(sub.Layers, copy)
	}
	return sub
}

// DemoComparison builds the three requested submissions and focused edge cases.
func DemoComparison() Comparison {
	scene := DemoScene()
	comparison := Comparison{Scene: scene}
	submissions := []Submission{
		emptyDepthSubmission(scene, "submission-no-depth"),
		sparseDepthSubmission(scene, "submission-sparse-depth", 0.002),
		fullDepthSubmission(scene, "submission-full-depth", 0.002),
	}
	for _, submission := range submissions {
		comparison.Submissions = append(comparison.Submissions, Composite(scene, submission, SafePolicy))
	}
	comparison.Diagnostics = DemoDiagnostics()
	return comparison
}

func onePixelScene(name string, layers []Layer) Scene {
	return Scene{Name: name, Width: 1, Height: 1, Layers: layers}
}

func onePixelSubmission(name string, mode DepthMode, step float64, layers []Layer) Submission {
	return Submission{Name: name, Mode: mode, Quantum: step, Layers: layers}
}

func copyForSubmission(layer Layer, left, right []float64) Layer {
	copy := cloneLayer(layer)
	copy.Left = left
	copy.Right = right
	return copy
}

func quantizationDiagnostic() Diagnostic {
	handTruth := Layer{
		ID: "hand", Kind: SurfaceHand, Order: 2,
		Coverage: []bool{true}, Left: []float64{0.6004}, Right: []float64{0.6004},
	}
	noticeTruth := Layer{
		ID: "notice", Kind: SurfaceNotification, System: true, Order: 0,
		Coverage: []bool{true}, Left: []float64{0.6012}, Right: []float64{0.6012},
	}
	scene := onePixelScene("near-ui-quantization", []Layer{noticeTruth, handTruth})
	const step = 0.003
	subLayers := []Layer{
		copyForSubmission(noticeTruth,
			[]float64{QuantizeDepth(noticeTruth.Left[0], step)},
			[]float64{QuantizeDepth(noticeTruth.Right[0], step)}),
		copyForSubmission(handTruth,
			[]float64{QuantizeDepth(handTruth.Left[0], step)},
			[]float64{QuantizeDepth(handTruth.Right[0], step)}),
	}
	sub := onePixelSubmission("near-ui-quantized", DepthFull, step, subLayers)
	safe := Composite(scene, sub, SafePolicy)
	bad := Composite(scene, sub, SystemFrontPolicy)
	return Diagnostic{
		Name:   "near-ui-quantization",
		Detail: "hand and system notification quantize to the same bin; stable explicit order keeps the hand in front",
		Safe:   safe, Bad: &bad,
	}
}

func invalidDepthDiagnostic() Diagnostic {
	handTruth := Layer{
		ID: "hand", Kind: SurfaceHand, Order: 2,
		Coverage: []bool{true}, Left: []float64{0.40}, Right: []float64{0.41},
	}
	panelTruth := Layer{
		ID: "panel", Kind: SurfacePanel, Order: 1,
		Coverage: []bool{true}, Left: []float64{1.0}, Right: []float64{1.0},
	}
	noticeTruth := Layer{
		ID: "notice", Kind: SurfaceNotification, System: true, Order: 0,
		Coverage: []bool{true}, Left: []float64{8.0}, Right: []float64{8.0},
	}
	scene := onePixelScene("invalid-hand-depth", []Layer{noticeTruth, panelTruth, handTruth})
	subLayers := []Layer{
		copyForSubmission(noticeTruth, noticeTruth.Left, noticeTruth.Right),
		copyForSubmission(panelTruth, panelTruth.Left, panelTruth.Right),
		copyForSubmission(handTruth, []float64{missingDepth()}, []float64{missingDepth()}),
	}
	sub := onePixelSubmission("invalid-hand-depth", DepthSparse, 0, subLayers)
	safe := Composite(scene, sub, SafePolicy)
	bad := Composite(scene, sub, IgnoreIncompletePolicy)
	return Diagnostic{
		Name:   "invalid-depth",
		Detail: "invalid hand depth is not treated as infinity; safe fallback uses explicit order, while ignoring the hole exposes the panel",
		Safe:   safe, Bad: &bad,
	}
}

func stereoDiagnostic() Diagnostic {
	handTruth := Layer{
		ID: "hand", Kind: SurfaceHand, Order: 2,
		Coverage: []bool{true}, Left: []float64{0.30}, Right: []float64{1.50},
	}
	panelTruth := Layer{
		ID: "panel", Kind: SurfacePanel, Order: 1,
		Coverage: []bool{true}, Left: []float64{1.0}, Right: []float64{1.0},
	}
	noticeTruth := Layer{
		ID: "notice", Kind: SurfaceNotification, System: true, Order: 0,
		Coverage: []bool{true}, Left: []float64{8.0}, Right: []float64{8.0},
	}
	scene := onePixelScene("stereo-depth-isolation", []Layer{noticeTruth, panelTruth, handTruth})
	subLayers := []Layer{
		copyForSubmission(noticeTruth, noticeTruth.Left, noticeTruth.Right),
		copyForSubmission(panelTruth, panelTruth.Left, panelTruth.Right),
		copyForSubmission(handTruth, handTruth.Left, handTruth.Right),
	}
	sub := onePixelSubmission("stereo-depth", DepthFull, 0, subLayers)
	safe := Composite(scene, sub, SafePolicy)
	bad := Composite(scene, sub, LeftEyeForBothPolicy)
	return Diagnostic{
		Name:   "stereo-isolation",
		Detail: "left eye sees the hand in front; right eye depth places the panel in front; reusing left depth corrupts the right eye",
		Safe:   safe, Bad: &bad,
	}
}

// DemoDiagnostics exercises the failure modes required by the compositor.
func DemoDiagnostics() []Diagnostic {
	return []Diagnostic{
		quantizationDiagnostic(),
		invalidDepthDiagnostic(),
		stereoDiagnostic(),
	}
}
