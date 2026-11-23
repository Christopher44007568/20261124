package gaze

import (
	"math"
)

// Sample is one cell of an eye's sampling map.
type Sample struct {
	Row     int   `json:"row"`
	Col     int   `json:"col"`
	Level   Level `json:"level"`
	Sampled bool  `json:"sampled"`
}

// Mask is one eye's independently generated sampling map.
//
// Tiers holds the quality level of every cell row-major. Boundary marks cells
// whose 4-neighborhood contains a different tier, i.e. the quality boundary
// that sharing is never allowed to move.
type Mask struct {
	Eye      Eye       `json:"eye"`
	Gaze     Point     `json:"gaze"`
	Occluded bool      `json:"occluded"`
	Size     int       `json:"size"`
	Tiers    [][]Level `json:"-"`
	Boundary [][]bool  `json:"-"`
	Params   Params    `json:"params"`
}

// BuildMask generates the sampling map of a single eye in isolation.
//
// Sharing never enters this function: the map is defined only by this eye's
// gaze point, occlusion state and quality parameters.
func BuildMask(cfg EyeConfig, size int) (*Mask, error) {
	if size <= 0 {
		return nil, errSize(size)
	}
	if err := cfg.Params.Validate(); err != nil {
		return nil, err
	}
	if err := checkPoint(cfg.Gaze); err != nil {
		return nil, err
	}

	tiers := make([][]Level, size)
	boundary := make([][]bool, size)
	for row := range tiers {
		tiers[row] = make([]Level, size)
		boundary[row] = make([]bool, size)
	}

	if cfg.Occluded {
		for row := range tiers {
			for col := range tiers[row] {
				tiers[row][col] = cfg.Params.FallbackLevel
			}
		}
	} else {
		for row := range tiers {
			for col := range tiers[row] {
				tiers[row][col] = levelAt(row, col, size, cfg.Gaze, cfg.Params)
			}
		}
	}

	fillBoundary(tiers, boundary)

	return &Mask{
		Eye:      cfg.Eye,
		Gaze:     cfg.Gaze,
		Occluded: cfg.Occluded,
		Size:     size,
		Tiers:    tiers,
		Boundary: boundary,
		Params:   cfg.Params,
	}, nil
}

// CountLevel returns the number of cells at the given quality tier.
func (m *Mask) CountLevel(level Level) int {
	count := 0
	for row := range m.Tiers {
		for col := range m.Tiers[row] {
			if m.Tiers[row][col] == level {
				count++
			}
		}
	}
	return count
}

// CountBoundary returns the number of cells on a quality boundary.
func (m *Mask) CountBoundary() int {
	count := 0
	for row := range m.Boundary {
		for _, marked := range m.Boundary[row] {
			if marked {
				count++
			}
		}
	}
	return count
}

// SampledCount counts cells that actually carry a sample.
func (m *Mask) SampledCount() int {
	return m.CountLevel(Foveal) + m.CountLevel(Parafoveal)
}

// Centroid returns the mean normalized coordinate of cells at the given tier.
// The boolean is false when no such cell exists.
func (m *Mask) Centroid(level Level) (Point, bool) {
	sumX, sumY, count := 0.0, 0.0, 0.0
	for row := range m.Tiers {
		for col, cell := range m.Tiers[row] {
			if cell != level {
				continue
			}
			sumX += (float64(col) + 0.5) / float64(m.Size)
			sumY += (float64(row) + 0.5) / float64(m.Size)
			count++
		}
	}
	if count == 0 {
		return Point{}, false
	}
	return Point{X: sumX / count, Y: sumY / count}, true
}

// Equals reports whether two maps are cell-for-cell identical.
func (m *Mask) Equals(other *Mask) bool {
	if other == nil || m.Size != other.Size {
		return false
	}
	for row := range m.Tiers {
		for col := range m.Tiers[row] {
			if m.Tiers[row][col] != other.Tiers[row][col] ||
				m.Boundary[row][col] != other.Boundary[row][col] {
				return false
			}
		}
	}
	return true
}

func levelAt(row, col, size int, gaze Point, params Params) Level {
	x := (float64(col) + 0.5) / float64(size)
	y := (float64(row) + 0.5) / float64(size)
	distance := math.Hypot(x-gaze.X, y-gaze.Y)
	switch {
	case distance <= params.FovealRadius:
		return Foveal
	case distance <= params.ParafovealRadius:
		return Parafoveal
	default:
		return Peripheral
	}
}

func fillBoundary(tiers [][]Level, boundary [][]bool) {
	size := len(tiers)
	for row := 0; row < size; row++ {
		for col := 0; col < size; col++ {
			current := tiers[row][col]
			for _, neighbor := range neighbors(row, col, size) {
				if tiers[neighbor[0]][neighbor[1]] != current {
					boundary[row][col] = true
					break
				}
			}
		}
	}
}

func neighbors(row, col, size int) [][2]int {
	var result [][2]int
	if row > 0 {
		result = append(result, [2]int{row - 1, col})
	}
	if row+1 < size {
		result = append(result, [2]int{row + 1, col})
	}
	if col > 0 {
		result = append(result, [2]int{row, col - 1})
	}
	if col+1 < size {
		result = append(result, [2]int{row, col + 1})
	}
	return result
}

func checkPoint(p Point) error {
	if p.X < 0 || p.X > 1 || p.Y < 0 || p.Y > 1 || math.IsNaN(p.X) || math.IsNaN(p.Y) {
		return errPoint(p)
	}
	return nil
}
