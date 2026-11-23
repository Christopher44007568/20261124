package gazedenoise

import (
	"fmt"
	"math"
	"strings"
)

// Metrics compares a filtered result with the clean truth. Halo values are
// signed mean luminance errors: positive is brighter than truth.
type Metrics struct {
	Mode string `json:"mode"`

	Pixels        int `json:"pixels"`
	Compared      int `json:"compared"`
	FilledHoles   int `json:"filled_holes"`
	UnfilledHoles int `json:"unfilled_holes"`

	ColourMAE  float64 `json:"colour_mae"`
	ColourRMSE float64 `json:"colour_rmse"`
	LumaMAE    float64 `json:"luma_mae"`

	CentreMAE    float64 `json:"centre_mae"`
	PeripheryMAE float64 `json:"periphery_mae"`
	BoundaryRMSE float64 `json:"boundary_rmse"`

	InnerHaloBias float64 `json:"inner_halo_bias"`
	OuterHaloBias float64 `json:"outer_halo_bias"`
	HaloBias      float64 `json:"halo_bias"`

	EdgeEnergyRatio          float64 `json:"edge_energy_ratio"`
	CentreEdgeEnergyRatio    float64 `json:"centre_edge_energy_ratio"`
	PeripheryEdgeEnergyRatio float64 `json:"periphery_edge_energy_ratio"`

	CrossPartitionTaps   int     `json:"cross_partition_taps"`
	CrossPartitionWeight float64 `json:"cross_partition_weight"`

	ProvenanceChecked     int                `json:"provenance_checked"`
	ProvenanceValid       bool               `json:"provenance_valid"`
	ProvenanceError       string             `json:"provenance_error,omitempty"`
	StressMAE             map[string]float64 `json:"stress_mae,omitempty"`
	StressPixels          map[string]int     `json:"stress_pixels,omitempty"`
	StressEdgeEnergyRatio map[string]float64 `json:"stress_edge_energy_ratio,omitempty"`
}

type errorAccumulator struct {
	n          int
	sumAbs     float64
	sumSq      float64
	sumLuma    float64
	sumAbsLuma float64
}

func (a *errorAccumulator) add(delta Vec3) {
	abs := (math.Abs(delta.X) + math.Abs(delta.Y) + math.Abs(delta.Z)) / 3
	sq := (delta.X*delta.X + delta.Y*delta.Y + delta.Z*delta.Z) / 3
	a.n++
	a.sumAbs += abs
	a.sumSq += sq
	luma := delta.Luma()
	a.sumLuma += luma
	a.sumAbsLuma += math.Abs(luma)
}

func (a errorAccumulator) mae() float64 {
	if a.n == 0 {
		return 0
	}
	return a.sumAbs / float64(a.n)
}

func (a errorAccumulator) rmse() float64 {
	if a.n == 0 {
		return 0
	}
	return math.Sqrt(a.sumSq / float64(a.n))
}

func (a errorAccumulator) meanLuma() float64 {
	if a.n == 0 {
		return 0
	}
	return a.sumLuma / float64(a.n)
}

func (a errorAccumulator) maeLuma() float64 {
	if a.n == 0 {
		return 0
	}
	return a.sumAbsLuma / float64(a.n)
}

// Evaluate computes reconstruction and audit metrics for one result.
func Evaluate(scene *Scene, result *Result) (Metrics, error) {
	if scene == nil || scene.Truth == nil || scene.Noisy == nil {
		return Metrics{}, fmt.Errorf("scene is incomplete")
	}
	if result == nil || result.Output == nil {
		return Metrics{}, fmt.Errorf("result is incomplete")
	}
	if err := scene.Truth.Validate(); err != nil {
		return Metrics{}, err
	}
	if err := result.Output.Validate(); err != nil {
		return Metrics{}, err
	}
	if scene.Truth.Width != result.Output.Width || scene.Truth.Height != result.Output.Height {
		return Metrics{}, fmt.Errorf("result dimensions differ from truth")
	}

	m := Metrics{
		Mode:                  result.Mode.String(),
		Pixels:                scene.Width * scene.Height,
		CrossPartitionTaps:    result.CrossPartitionTaps,
		CrossPartitionWeight:  result.CrossPartitionWeight,
		StressMAE:             make(map[string]float64),
		StressPixels:          make(map[string]int),
		StressEdgeEnergyRatio: make(map[string]float64),
	}
	var all, centre, periphery, boundary, innerHalo, outerHalo errorAccumulator

	for i := range scene.Truth.Color {
		if !result.Output.Valid[i] {
			continue
		}
		delta := result.Output.Color[i].Sub(scene.Truth.Color[i])
		all.add(delta)
		if scene.Truth.Mask[i] {
			centre.add(delta)
		} else {
			periphery.add(delta)
		}
		if i < len(scene.BoundaryDistance) {
			d := scene.BoundaryDistance[i]
			if d <= 2.5 {
				boundary.add(delta)
			}
			if d <= 2.0 {
				if scene.Truth.Mask[i] {
					innerHalo.add(delta)
				} else {
					outerHalo.add(delta)
				}
			}
		}
	}
	m.Compared = all.n
	m.ColourMAE = all.mae()
	m.ColourRMSE = all.rmse()
	m.LumaMAE = all.maeLuma()
	m.CentreMAE = centre.mae()
	m.PeripheryMAE = periphery.mae()
	m.BoundaryRMSE = boundary.rmse()
	m.InnerHaloBias = innerHalo.meanLuma()
	m.OuterHaloBias = outerHalo.meanLuma()
	m.HaloBias = 0.5 * (math.Abs(m.InnerHaloBias) + math.Abs(m.OuterHaloBias))

	m.EdgeEnergyRatio, m.CentreEdgeEnergyRatio, m.PeripheryEdgeEnergyRatio =
		edgeEnergyRatios(scene.Truth, result.Output)

	for _, name := range scene.FeatureNames() {
		mask := scene.Feature(name)
		if len(mask) != len(scene.Truth.Color) {
			continue
		}
		var feature errorAccumulator
		for i, selected := range mask {
			if !selected || !result.Output.Valid[i] {
				continue
			}
			feature.add(result.Output.Color[i].Sub(scene.Truth.Color[i]))
		}
		m.StressPixels[name] = feature.n
		m.StressMAE[name] = feature.mae()
		ratio, _, _ := edgeEnergyRatiosMasked(scene.Truth, result.Output, mask)
		m.StressEdgeEnergyRatio[name] = ratio
	}

	validateProvenance(scene, result, &m)
	return m, nil
}

func edgeEnergyRatios(truth, out *Frame) (all, centre, periphery float64) {
	return edgeEnergyRatiosMasked(truth, out, nil)
}

func edgeEnergyRatiosMasked(truth, out *Frame, include []bool) (all, centre, periphery float64) {
	var truthAll, outAll, truthCentre, outCentre, truthPeriphery, outPeriphery float64
	for y := 1; y < truth.Height-1; y++ {
		for x := 1; x < truth.Width-1; x++ {
			i := truth.Index(x, y)
			if include != nil && (i >= len(include) || !include[i]) {
				continue
			}
			if !validNeighbourhood(truth, x, y) || !validNeighbourhood(out, x, y) {
				continue
			}
			gt := gradientEnergy(truth, x, y)
			goe := gradientEnergy(out, x, y)
			truthAll += gt
			outAll += goe
			if truth.Mask[i] {
				truthCentre += gt
				outCentre += goe
			} else {
				truthPeriphery += gt
				outPeriphery += goe
			}
		}
	}
	return ratio(outAll, truthAll), ratio(outCentre, truthCentre), ratio(outPeriphery, truthPeriphery)
}

func validNeighbourhood(f *Frame, x, y int) bool {
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if !f.Valid[f.Index(x+dx, y+dy)] {
				return false
			}
		}
	}
	return true
}

func gradientEnergy(f *Frame, x, y int) float64 {
	i := f.Index(x, y)
	gx := (f.Luma[i+1] - f.Luma[i-1]) * 0.5
	gy := (f.Luma[i+f.Width] - f.Luma[i-f.Width]) * 0.5
	return gx*gx + gy*gy
}

func ratio(out, truth float64) float64 {
	if truth <= 1e-15 {
		if out <= 1e-15 {
			return 1
		}
		return 0
	}
	return out / truth
}

func validateProvenance(scene *Scene, result *Result, m *Metrics) {
	m.ProvenanceValid = true
	m.FilledHoles = 0
	for _, filled := range result.Filled {
		if filled {
			m.FilledHoles++
		}
	}
	m.UnfilledHoles = result.UnfilledHoles

	byPixel := make(map[int]HoleProvenance, len(result.Provenance))
	var problems []string
	for _, prov := range result.Provenance {
		if prov.Pixel < 0 || prov.Pixel >= len(scene.Noisy.Color) {
			problems = append(problems, fmt.Sprintf("provenance pixel %d out of range", prov.Pixel))
			continue
		}
		if _, exists := byPixel[prov.Pixel]; exists {
			problems = append(problems, fmt.Sprintf("duplicate provenance for pixel %d", prov.Pixel))
		}
		byPixel[prov.Pixel] = prov
		m.ProvenanceChecked++

		if scene.Noisy.Valid[prov.Pixel] {
			problems = append(problems, fmt.Sprintf("pixel %d was valid but has hole provenance", prov.Pixel))
		}
		if !result.Filled[prov.Pixel] {
			problems = append(problems, fmt.Sprintf("pixel %d has provenance but is not marked filled", prov.Pixel))
		}
		if len(prov.Sources) == 0 {
			problems = append(problems, fmt.Sprintf("pixel %d has no source pixels", prov.Pixel))
			continue
		}

		var totalWeight float64
		var reconstructed Vec3
		cross := false
		for _, source := range prov.Sources {
			if source.Pixel < 0 || source.Pixel >= len(scene.Noisy.Color) {
				problems = append(problems, fmt.Sprintf("pixel %d source %d out of range", prov.Pixel, source.Pixel))
				continue
			}
			if source.X != source.Pixel%scene.Width || source.Y != source.Pixel/scene.Width {
				problems = append(problems, fmt.Sprintf("pixel %d source %d has wrong coordinates", prov.Pixel, source.Pixel))
			}
			if !scene.Noisy.Valid[source.Pixel] {
				problems = append(problems, fmt.Sprintf("pixel %d uses invalid source %d", prov.Pixel, source.Pixel))
			}
			if source.Weight <= 0 || source.Weight > 1 {
				problems = append(problems, fmt.Sprintf("pixel %d source %d has invalid weight %v",
					prov.Pixel, source.Pixel, source.Weight))
			}
			if result.Mode == ModePartitioned && scene.Noisy.Mask[source.Pixel] != scene.Noisy.Mask[prov.Pixel] {
				problems = append(problems, fmt.Sprintf("pixel %d borrowed source %d across gaze boundary",
					prov.Pixel, source.Pixel))
			}
			if scene.Noisy.Mask[source.Pixel] != scene.Noisy.Mask[prov.Pixel] {
				cross = true
			}
			totalWeight += source.Weight
			reconstructed = reconstructed.Add(scene.Noisy.Color[source.Pixel].Mul(source.Weight))
		}
		if math.Abs(totalWeight-1) > 1e-9 {
			problems = append(problems, fmt.Sprintf("pixel %d source weights sum to %.12f", prov.Pixel, totalWeight))
		}
		if prov.CrossPartition != cross {
			problems = append(problems, fmt.Sprintf("pixel %d cross-partition flag is inconsistent", prov.Pixel))
		}
		if result.Output.Valid[prov.Pixel] && reconstructed.Sub(result.Output.Color[prov.Pixel]).Length() > 1e-9 {
			problems = append(problems, fmt.Sprintf("pixel %d value does not match its recorded sources", prov.Pixel))
		}
	}

	for i, filled := range result.Filled {
		if !filled {
			continue
		}
		if _, ok := byPixel[i]; !ok {
			problems = append(problems, fmt.Sprintf("filled pixel %d has no provenance record", i))
		}
	}
	if len(problems) > 0 {
		m.ProvenanceValid = false
		m.ProvenanceError = strings.Join(problems, "; ")
	}
}
