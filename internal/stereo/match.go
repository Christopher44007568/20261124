package stereo

import (
	"image"
	"math"
	"sort"
)

type StereoDepth struct {
	Width         int
	Height        int
	Valid         []bool
	Disparity     []float64
	Depth         []float64
	WorldX        []float64
	WorldY        []float64
	LeftCost      []float64
	ValidCount    int
	MeanMatchCost float64
}

func RecoverDepth(left, right *image.RGBA, ipd float64) StereoDepth {
	maxDisp := int(math.Ceil(FocalPixels*ipd/MinSceneZ)) + 2
	minDisp := int(math.Floor(FocalPixels*ipd/(WallZ+100))) - 2
	if minDisp < 0 {
		minDisp = 0
	}
	leftDisp, leftCost, _ := blockMatch(left, right, minDisp, maxDisp, -1)
	rightDisp, _, _ := blockMatch(right, left, minDisp, maxDisp, 1)
	leftDisp = medianDisparityFilter(leftDisp, 2, 3)
	rightDisp = medianDisparityFilter(rightDisp, 2, 3)

	result := StereoDepth{
		Width:     ImageWidth,
		Height:    ImageHeight,
		Valid:     make([]bool, ImageWidth*ImageHeight),
		Disparity: make([]float64, ImageWidth*ImageHeight),
		Depth:     make([]float64, ImageWidth*ImageHeight),
		WorldX:    make([]float64, ImageWidth*ImageHeight),
		WorldY:    make([]float64, ImageWidth*ImageHeight),
		LeftCost:  leftCost,
	}
	for idx := range result.Depth {
		result.Depth[idx] = math.Inf(1)
	}

	const radius = 2
	var costSum float64
	for y := radius; y < ImageHeight-radius; y++ {
		for x := radius; x < ImageWidth-radius; x++ {
			idx := y*ImageWidth + x
			d := float64(leftDisp[idx])
			if d < 0.5 {
				continue
			}
			xr := int(math.Round(float64(x) - d))
			if xr < radius || xr >= ImageWidth-radius {
				continue
			}
			ridx := y*ImageWidth + xr
			if rightDisp[ridx] < 1 || math.Abs(float64(rightDisp[ridx])-d) > 1.35 {
				continue
			}

			refined := refineDisparity(left, right, x, y, int(math.Round(d)), radius)
			if refined < 0.5 {
				continue
			}
			u := float64(x) + 0.5
			z := FocalPixels * ipd / refined
			if z <= 0 || math.IsInf(z, 0) {
				continue
			}
			result.Valid[idx] = true
			result.ValidCount++
			result.Disparity[idx] = refined
			result.Depth[idx] = z
			result.WorldX[idx] = (u - refined/2 - PrincipalX) * z / FocalPixels
			result.WorldY[idx] = (PrincipalY - (float64(y) + 0.5)) * z / FocalPixels
			costSum += leftCost[idx]
		}
	}
	if result.ValidCount != 0 {
		result.MeanMatchCost = costSum / float64(result.ValidCount)
	}
	return result
}

// blockMatch returns integer disparities for pixels in base. direction is -1
// when matching left-to-right and +1 when matching right-to-left.
func blockMatch(base, other *image.RGBA, minDisp, maxDisp, direction int) ([]int, []float64, []float64) {
	const radius = 2
	const diameter = 2*radius + 1
	const area = float64(diameter * diameter)
	n := ImageWidth * ImageHeight
	bestD := make([]int, n)
	bestCost := make([]float64, n)
	secondCost := make([]float64, n)
	for i := 0; i < n; i++ {
		bestD[i] = -1
		bestCost[i] = math.Inf(1)
		secondCost[i] = math.Inf(1)
	}

	horizontal := make([]float64, n)
	for d := minDisp; d <= maxDisp; d++ {
		offset := direction * d
		for y := 0; y < ImageHeight; y++ {
			for x := 0; x < ImageWidth; x++ {
				idx := y*ImageWidth + x
				ox := x + offset
				if x-radius < 0 || x+radius >= ImageWidth || ox-radius < 0 || ox+radius >= ImageWidth {
					horizontal[idx] = math.Inf(1)
					continue
				}
				var sum float64
				for k := -radius; k <= radius; k++ {
					sum += pixelCost(base, idx+k, other, y*ImageWidth+ox+k)
				}
				horizontal[idx] = sum
			}
		}

		for x := radius; x < ImageWidth-radius; x++ {
			ox := x + offset
			if ox-radius < 0 || ox+radius >= ImageWidth {
				continue
			}
			var vertical float64
			for yy := 0; yy <= 2*radius; yy++ {
				vertical += horizontal[yy*ImageWidth+x]
			}
			for y := radius; y < ImageHeight-radius; y++ {
				if y > radius {
					vertical += horizontal[(y+radius)*ImageWidth+x]
					vertical -= horizontal[(y-radius-1)*ImageWidth+x]
				}
				cost := vertical / area
				idx := y*ImageWidth + x
				if cost < bestCost[idx] {
					if bestD[idx] >= 0 && absInt(d-bestD[idx]) > 1 {
						if bestCost[idx] < secondCost[idx] {
							secondCost[idx] = bestCost[idx]
						}
					}
					bestCost[idx] = cost
					bestD[idx] = d
				} else if bestD[idx] >= 0 && d > bestD[idx]+1 && cost < secondCost[idx] {
					secondCost[idx] = cost
				}
			}
		}
	}

	for i := range bestCost {
		if bestD[i] >= 0 && secondCost[i] < math.Inf(1) && secondCost[i] < bestCost[i]*1.05 {
			bestD[i] = -1
		}
	}
	return bestD, bestCost, secondCost
}

func pixelCost(a *image.RGBA, ia int, b *image.RGBA, ib int) float64 {
	ao, bo := ia*4, ib*4
	dr := (float64(a.Pix[ao]) - float64(b.Pix[bo])) / 255
	dg := (float64(a.Pix[ao+1]) - float64(b.Pix[bo+1])) / 255
	db := (float64(a.Pix[ao+2]) - float64(b.Pix[bo+2])) / 255
	return (dr*dr + dg*dg + db*db) / 3
}

func blockCostAt(base, other *image.RGBA, x, y, d, radius int) float64 {
	if x-radius < 0 || x+radius >= ImageWidth || y-radius < 0 || y+radius >= ImageHeight {
		return math.Inf(1)
	}
	ox := x - d
	if ox-radius < 0 || ox+radius >= ImageWidth {
		return math.Inf(1)
	}
	var sum float64
	for dy := -radius; dy <= radius; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			sum += pixelCost(base, (y+dy)*ImageWidth+x+dx, other, (y+dy)*ImageWidth+ox+dx)
		}
	}
	return sum / float64((2*radius+1)*(2*radius+1))
}

func refineDisparity(base, other *image.RGBA, x, y, d, radius int) float64 {
	c0 := blockCostAt(base, other, x, y, d, radius)
	if math.IsInf(c0, 0) {
		return float64(d)
	}
	cm := blockCostAt(base, other, x, y, d-1, radius)
	cp := blockCostAt(base, other, x, y, d+1, radius)
	if math.IsInf(cm, 0) || math.IsInf(cp, 0) {
		return float64(d)
	}
	denom := cm - 2*c0 + cp
	if math.Abs(denom) < 1e-12 {
		return float64(d)
	}
	delta := 0.5 * (cm - cp) / denom
	if delta > 0.5 {
		delta = 0.5
	} else if delta < -0.5 {
		delta = -0.5
	}
	return float64(d) + delta
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func medianDisparityFilter(input []int, radius, tolerance int) []int {
	output := append([]int(nil), input...)
	scratch := make([]int, 0, (2*radius+1)*(2*radius+1))
	for y := radius; y < ImageHeight-radius; y++ {
		for x := radius; x < ImageWidth-radius; x++ {
			idx := y*ImageWidth + x
			d := input[idx]
			if d < 0 {
				continue
			}
			scratch = scratch[:0]
			for yy := y - radius; yy <= y+radius; yy++ {
				for xx := x - radius; xx <= x+radius; xx++ {
					if v := input[yy*ImageWidth+xx]; v >= 0 {
						scratch = append(scratch, v)
					}
				}
			}
			sort.Ints(scratch)
			median := scratch[len(scratch)/2]
			if absInt(d-median) > tolerance {
				output[idx] = -1
			}
		}
	}
	return output
}
