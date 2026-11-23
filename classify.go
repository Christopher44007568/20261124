package main

// Class is the stereo visibility class of one instance.
type Class int

const (
	ClassBoth Class = iota
	ClassLeftOnly
	ClassRightOnly
	ClassNeither
)

func (c Class) String() string {
	switch c {
	case ClassBoth:
		return "双眼"
	case ClassLeftOnly:
		return "仅左"
	case ClassRightOnly:
		return "仅右"
	default:
		return "双眼不可见"
	}
}

// Eye index convention: 0 = left, 1 = right.
const (
	EyeLeft  = 0
	EyeRight = 1
)

var eyeNames = [2]string{"左眼", "右眼"}

// Record is the classification result of one instance, with per-eye evidence.
type Record struct {
	Inst     *Instance
	Class    Class
	Evidence [2]EyeEvidence
}

// Counts tallies the four classes; Total must equal the input instance count.
type Counts struct {
	Both, LeftOnly, RightOnly, Neither, Total int
}

func (c Counts) Sum() int { return c.Both + c.LeftOnly + c.RightOnly + c.Neither }

// Closed reports whether the four class counts close to the input total.
func (c Counts) Closed() bool { return c.Sum() == c.Total }

func classOf(left, right bool) Class {
	switch {
	case left && right:
		return ClassBoth
	case left:
		return ClassLeftOnly
	case right:
		return ClassRightOnly
	default:
		return ClassNeither
	}
}

// ClassifyAll culls every bound instance against both eyes' frusta and
// returns one record per instance plus counts closed to the input total.
func ClassifyAll(bound []BoundInstance, frusta [2]Frustum) ([]Record, Counts) {
	records := make([]Record, 0, len(bound))
	counts := Counts{Total: len(bound)}
	for _, b := range bound {
		center, axes, half := b.WorldOBB()
		rec := Record{Inst: b.Inst}
		for eye := 0; eye < 2; eye++ {
			rec.Evidence[eye] = CullOBB(frusta[eye], center, axes, half)
		}
		rec.Class = classOf(rec.Evidence[EyeLeft].Visible, rec.Evidence[EyeRight].Visible)
		switch rec.Class {
		case ClassBoth:
			counts.Both++
		case ClassLeftOnly:
			counts.LeftOnly++
		case ClassRightOnly:
			counts.RightOnly++
		case ClassNeither:
			counts.Neither++
		}
		records = append(records, rec)
	}
	return records, counts
}

// DrawList is the lightweight per-eye draw list: IDs of instances the
// classifier considers visible for that eye, in scene order.
func DrawList(records []Record, eye int) []int {
	var ids []int
	for _, r := range records {
		if r.Evidence[eye].Visible {
			ids = append(ids, r.Inst.ID)
		}
	}
	return ids
}
