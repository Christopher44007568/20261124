package stereocull

import "math/bits"

// IDSet is a dense bit set keyed by instance ID. It supports the same
// membership/set operations as a map[int]bool but avoids per-insert hashing
// and allocation during hot cull loops.
type IDSet struct {
	bits []uint64
}

// NewIDSet returns a set sized for n distinct IDs.
func NewIDSet(n int) IDSet {
	return IDSet{bits: make([]uint64, (n+63)/64)}
}

// Add marks id as present.
func (s IDSet) Add(id int) { s.bits[id>>6] |= 1 << (uint(id) & 63) }

// Has reports whether id is present.
func (s IDSet) Has(id int) bool {
	word := id >> 6
	return word < len(s.bits) && s.bits[word]&(1<<(uint(id)&63)) != 0
}

// Len returns the number of marked IDs.
func (s IDSet) Len() int {
	n := 0
	for _, word := range s.bits {
		n += popcount(word)
	}
	return n
}

// Clone returns an independent copy.
func (s IDSet) Clone() IDSet {
	out := IDSet{bits: append([]uint64(nil), s.bits...)}
	return out
}

// Equal reports whether the two sets contain the same IDs.
func (s IDSet) Equal(o IDSet) bool {
	if len(s.bits) != len(o.bits) {
		return false
	}
	for i := range s.bits {
		if s.bits[i] != o.bits[i] {
			return false
		}
	}
	return true
}

// IDs appends all marked IDs to dst and returns the resulting slice.
func (s IDSet) IDs(dst []int) []int {
	for i, word := range s.bits {
		for word != 0 {
			b := trailingZeros(word)
			dst = append(dst, i*64+b)
			word &= word - 1
		}
	}
	return dst
}

// UnionSet returns a new set containing IDs from either operand.
func UnionSet(a, b IDSet) IDSet {
	out := NewIDSet(len(a.bits) * 64)
	for i := range out.bits {
		out.bits[i] = a.bits[i] | b.bits[i]
	}
	return out
}

// IntersectSet returns a new set containing IDs present in both operands.
func IntersectSet(a, b IDSet) IDSet {
	out := NewIDSet(len(a.bits) * 64)
	for i := range out.bits {
		out.bits[i] = a.bits[i] & b.bits[i]
	}
	return out
}

// DiffSet returns IDs in a that are not in b.
func DiffSet(a, b IDSet) IDSet {
	out := NewIDSet(len(a.bits) * 64)
	for i := range out.bits {
		out.bits[i] = a.bits[i] &^ b.bits[i]
	}
	return out
}

func popcount(x uint64) int {
	return bits.OnesCount64(x)
}

func trailingZeros(x uint64) int {
	return bits.TrailingZeros64(x)
}
