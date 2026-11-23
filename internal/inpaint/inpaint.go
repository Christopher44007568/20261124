// Package inpaint provides deterministic, dependency-free reconstructors used
// by the benchmark. They intentionally represent different global/center
// quality trade-offs.
package inpaint

import (
	"fmt"

	"fidelitycheck/internal/imaging"
)

type Algorithm struct {
	Name        string
	Description string
	Reconstruct func(*imaging.Frame, *imaging.Mask) *imaging.Frame
}

func Defaults() []Algorithm {
	return []Algorithm{
		{
			Name:        "mean",
			Description: "fill every missing pixel with the global valid-pixel mean",
			Reconstruct: MeanFill,
		},
		{
			Name:        "nearest",
			Description: "propagate the nearest observed color into the mask",
			Reconstruct: NearestFill,
		},
		{
			Name:        "blur",
			Description: "weighted local color fill followed by masked smoothing",
			Reconstruct: BlurFill,
		},
		{
			Name:        "diffusion",
			Description: "harmonic diffusion from the mask boundary",
			Reconstruct: DiffusionFill,
		},
	}
}

func MeanFill(src *imaging.Frame, mask *imaging.Mask) *imaging.Frame {
	out := src.Clone()
	mean, ok := validMean(src, mask)
	if !ok {
		mean = imaging.RGB{R: 0.5, G: 0.5, B: 0.5}
	}
	for i, missing := range mask.Bits {
		if missing {
			out.Pix[i] = mean
		}
	}
	return out
}

func NearestFill(src *imaging.Frame, mask *imaging.Mask) *imaging.Frame {
	out := src.Clone()
	n := src.W * src.H
	dist := make([]int, n)
	queue := make([]int, 0, n)
	for i := 0; i < n; i++ {
		dist[i] = -1
		if !mask.Bits[i] {
			dist[i] = 0
			queue = append(queue, i)
		}
	}
	if len(queue) == 0 {
		out.Fill(imaging.RGB{R: 0.5, G: 0.5, B: 0.5})
		return out
	}

	directions := [4][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}
	for head := 0; head < len(queue); head++ {
		i := queue[head]
		x, y := i%src.W, i/src.W
		for _, d := range directions {
			nx, ny := x+d[0], y+d[1]
			if nx < 0 || ny < 0 || nx >= src.W || ny >= src.H {
				continue
			}
			j := ny*src.W + nx
			if dist[j] != -1 {
				continue
			}
			dist[j] = dist[i] + 1
			out.Pix[j] = out.Pix[i]
			queue = append(queue, j)
		}
	}
	return out
}

func BlurFill(src *imaging.Frame, mask *imaging.Mask) *imaging.Frame {
	out := src.Clone()
	fallback, ok := validMean(src, mask)
	if !ok {
		fallback = imaging.RGB{R: 0.5, G: 0.5, B: 0.5}
	}

	const radius = 4
	for i, missing := range mask.Bits {
		if !missing {
			continue
		}
		x, y := i%src.W, i/src.W
		var sum imaging.RGB
		var weight float64
		for oy := -radius; oy <= radius; oy++ {
			for ox := -radius; ox <= radius; ox++ {
				px, py := x+ox, y+oy
				if px < 0 || py < 0 || px >= src.W || py >= src.H {
					continue
				}
				j := py*src.W + px
				if mask.Bits[j] {
					continue
				}
				d2 := float64(ox*ox + oy*oy)
				w := 1 / (1 + d2)
				sum = sum.Add(src.Pix[j].Scale(w))
				weight += w
			}
		}
		if weight == 0 {
			out.Pix[i] = fallback
		} else {
			out.Pix[i] = sum.Scale(1 / weight)
		}
	}

	for pass := 0; pass < 3; pass++ {
		next := out.Clone()
		for i, missing := range mask.Bits {
			if !missing {
				continue
			}
			x, y := i%src.W, i/src.W
			var sum imaging.RGB
			var count float64
			for oy := -1; oy <= 1; oy++ {
				for ox := -1; ox <= 1; ox++ {
					px, py := x+ox, y+oy
					if px < 0 || py < 0 || px >= src.W || py >= src.H {
						continue
					}
					sum = sum.Add(out.At(px, py))
					count++
				}
			}
			next.Pix[i] = sum.Scale(1 / count)
		}
		out = next
	}
	return out
}

func DiffusionFill(src *imaging.Frame, mask *imaging.Mask) *imaging.Frame {
	out := src.Clone()
	mean, ok := validMean(src, mask)
	if !ok {
		mean = imaging.RGB{R: 0.5, G: 0.5, B: 0.5}
	}
	for i, missing := range mask.Bits {
		if missing {
			out.Pix[i] = mean
		}
	}

	const iterations = 120
	const omega = 1.85
	for iter := 0; iter < iterations; iter++ {
		for y := 0; y < src.H; y++ {
			for x := 0; x < src.W; x++ {
				i := y*src.W + x
				if !mask.Bits[i] {
					continue
				}
				var sum imaging.RGB
				var count float64
				if x > 0 {
					sum = sum.Add(out.Pix[i-1])
					count++
				}
				if x+1 < src.W {
					sum = sum.Add(out.Pix[i+1])
					count++
				}
				if y > 0 {
					sum = sum.Add(out.Pix[i-src.W])
					count++
				}
				if y+1 < src.H {
					sum = sum.Add(out.Pix[i+src.W])
					count++
				}
				if count == 0 {
					continue
				}
				avg := sum.Scale(1 / count)
				old := out.Pix[i]
				delta := avg.Sub(old).Scale(omega)
				out.Pix[i] = old.Add(delta).Clamp()
			}
		}
	}
	return out
}

func validMean(f *imaging.Frame, mask *imaging.Mask) (imaging.RGB, bool) {
	var sum imaging.RGB
	var count float64
	for i, missing := range mask.Bits {
		if !missing {
			sum = sum.Add(f.Pix[i])
			count++
		}
	}
	if count == 0 {
		return imaging.RGB{}, false
	}
	return sum.Scale(1 / count), true
}

func Find(algorithms []Algorithm, name string) (Algorithm, error) {
	for _, alg := range algorithms {
		if alg.Name == name {
			return alg, nil
		}
	}
	return Algorithm{}, fmt.Errorf("unknown reconstructor %q", name)
}

func Names(algorithms []Algorithm) []string {
	names := make([]string, len(algorithms))
	for i, alg := range algorithms {
		names[i] = alg.Name
	}
	return names
}
