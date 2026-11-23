package gaze

import "errors"

// Cell identifies one grid cell.
type Cell struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

// Report records the overlap region, the safe shared mask and the work plan.
//
// Overlap marks cells sampled by both eyes. Shared marks the subset that may
// be computed once: both eyes are visible, the tier matches and the cell sits
// on neither eye's quality boundary.
type Report struct {
	Size              int      `json:"size"`
	Overlap           [][]bool `json:"-"`
	Shared            [][]bool `json:"-"`
	OverlapCount      int      `json:"overlapCount"`
	SharedCount       int      `json:"sharedCount"`
	LeftWork          int      `json:"leftWork"`
	RightWork         int      `json:"rightWork"`
	StandaloneWork    int      `json:"standaloneWork"`
	SharedWork        int      `json:"sharedWork"`
	LeftPrivateCount  int      `json:"leftPrivateCount"`
	RightPrivateCount int      `json:"rightPrivateCount"`
	EitherOccluded    bool     `json:"eitherOccluded"`
}

// WorkPlan is the execution plan after sharing is applied.
type WorkPlan struct {
	Shared       []Cell `json:"shared"`
	LeftPrivate  []Cell `json:"leftPrivate"`
	RightPrivate []Cell `json:"rightPrivate"`
}

// AnalyzeSharing compares two independently generated eye maps.
func AnalyzeSharing(left, right *Mask) (*Report, error) {
	if left == nil || right == nil {
		return nil, errors.New("gaze: AnalyzeSharing requires both eye masks")
	}
	if left.Size != right.Size {
		return nil, errors.New("gaze: eye masks must share the same grid size")
	}

	size := left.Size
	overlap := make([][]bool, size)
	shared := make([][]bool, size)
	for row := range overlap {
		overlap[row] = make([]bool, size)
		shared[row] = make([]bool, size)
	}

	occluded := left.Occluded || right.Occluded
	report := &Report{
		Size:           size,
		Overlap:        overlap,
		Shared:         shared,
		LeftWork:       left.SampledCount(),
		RightWork:      right.SampledCount(),
		EitherOccluded: occluded,
	}

	for row := 0; row < size; row++ {
		for col := 0; col < size; col++ {
			leftSampled := left.Tiers[row][col] != Peripheral
			rightSampled := right.Tiers[row][col] != Peripheral
			if !(leftSampled && rightSampled) {
				continue
			}
			overlap[row][col] = true
			report.OverlapCount++

			if shareable(left, right, row, col, occluded) {
				shared[row][col] = true
				report.SharedCount++
			}
		}
	}

	report.StandaloneWork = report.LeftWork + report.RightWork
	report.SharedWork = report.StandaloneWork - report.SharedCount
	report.LeftPrivateCount = report.LeftWork - report.SharedCount
	report.RightPrivateCount = report.RightWork - report.SharedCount
	return report, nil
}

// IsShared reports whether a cell is evaluated once for both eyes.
func (r *Report) IsShared(row, col int) bool {
	return r.Shared[row][col]
}

// PlanWork lists which cells are shared and which stay private per eye.
func (r *Report) PlanWork() WorkPlan {
	plan := WorkPlan{}
	for row := 0; row < r.Size; row++ {
		for col := 0; col < r.Size; col++ {
			if r.Shared[row][col] {
				plan.Shared = append(plan.Shared, Cell{Row: row, Col: col})
				continue
			}
			if r.Overlap[row][col] {
				plan.LeftPrivate = append(plan.LeftPrivate, Cell{Row: row, Col: col})
				plan.RightPrivate = append(plan.RightPrivate, Cell{Row: row, Col: col})
			}
		}
	}
	return plan
}

// ApplySharing simulates the optimized pipeline.
//
// Shared cells are evaluated once from the left map and reused by the right
// eye; every other cell keeps its own eye's evaluation. The returned maps must
// be cell-for-cell identical to standalone maps, which proves sharing only
// removes work and never moves a monocular quality boundary.
func ApplySharing(left, right *Mask, report *Report) (*Mask, *Mask) {
	rebuiltLeft := cloneMask(left)
	rebuiltRight := cloneMask(right)
	for row := 0; row < report.Size; row++ {
		for col := 0; col < report.Size; col++ {
			if !report.Shared[row][col] {
				continue
			}
			level := left.Tiers[row][col]
			rebuiltLeft.Tiers[row][col] = level
			rebuiltRight.Tiers[row][col] = level
		}
	}
	fillBoundary(rebuiltLeft.Tiers, rebuiltLeft.Boundary)
	fillBoundary(rebuiltRight.Tiers, rebuiltRight.Boundary)
	return rebuiltLeft, rebuiltRight
}

func shareable(left, right *Mask, row, col int, occluded bool) bool {
	if occluded {
		return false
	}
	if left.Tiers[row][col] != right.Tiers[row][col] {
		return false
	}
	if left.Boundary[row][col] || right.Boundary[row][col] {
		return false
	}
	return true
}

func cloneMask(source *Mask) *Mask {
	tiers := make([][]Level, source.Size)
	boundary := make([][]bool, source.Size)
	for row := range source.Tiers {
		tiers[row] = append([]Level(nil), source.Tiers[row]...)
		boundary[row] = append([]bool(nil), source.Boundary[row]...)
	}
	return &Mask{
		Eye:      source.Eye,
		Gaze:     source.Gaze,
		Occluded: source.Occluded,
		Size:     source.Size,
		Tiers:    tiers,
		Boundary: boundary,
		Params:   source.Params,
	}
}
