package strided

import "fmt"

// ErrKind classifies decoder failures so callers can branch on them.
type ErrKind int

const (
	ErrEmptyImage    ErrKind = iota // width or height is not positive
	ErrInvalidStride                // |stride| smaller than one packed row
	ErrOutOfBounds                  // a row falls outside the backing buffer
	ErrOverflow                     // dimension arithmetic overflows int64
)

func (k ErrKind) String() string {
	switch k {
	case ErrEmptyImage:
		return "empty image"
	case ErrInvalidStride:
		return "invalid stride"
	case ErrOutOfBounds:
		return "out of bounds"
	case ErrOverflow:
		return "integer overflow"
	default:
		return "unknown error"
	}
}

// Error pinpoints where decoding failed: the row being decoded and the
// byte offset into the source buffer that was required. Row and Offset
// are -1 when not applicable.
type Error struct {
	Kind   ErrKind
	Format Format
	Row    int
	Offset int64
	Detail string
}

func (e *Error) Error() string {
	loc := ""
	if e.Row >= 0 {
		loc = fmt.Sprintf(" row=%d", e.Row)
	}
	if e.Offset >= 0 {
		loc += fmt.Sprintf(" offset=%d", e.Offset)
	}
	if e.Detail != "" {
		return fmt.Sprintf("strided: %s (%s):%s %s", e.Kind, e.Format, loc, e.Detail)
	}
	return fmt.Sprintf("strided: %s (%s):%s", e.Kind, e.Format, loc)
}
