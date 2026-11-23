package depthlife

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
)

// Template is the saved artifact of one eye on one machine.
type Template struct {
	Machine   Machine
	Eye       Eye
	AttID     string
	Version   uint32
	Width     uint16
	Height    uint16
	Z         []uint16
	Threshold uint16
	Coverage  float64
	Passed    bool
	Mask      []bool
}

func (t *Template) TaggedEye() Eye           { return t.Eye }
func (t *Template) TaggedAttachment() string { return t.AttID }

var templateMagic = [4]byte{'D', 'L', 'T', 'P'}

// magic(4) + machine(1) + eye(1) + attID(32) + version(4) +
// width(2) + height(2) + threshold(2) + coverage(8) + passed(1)
const templateHeaderSize = 57

// Encode serializes a template to a self-describing blob (big-endian).
func (t *Template) Encode() ([]byte, error) {
	if len(t.AttID) != 64 {
		return nil, fmt.Errorf("%w: attachment id", ErrBadBlob)
	}
	if _, err := hex.DecodeString(t.AttID); err != nil {
		return nil, fmt.Errorf("%w: attachment id: %v", ErrBadBlob, err)
	}
	n := int(t.Width) * int(t.Height)
	if len(t.Z) != n || len(t.Mask) != n {
		return nil, fmt.Errorf("%w: dimension/sample mismatch", ErrBadBlob)
	}
	attID, _ := hex.DecodeString(t.AttID)
	maskBytes := (n + 7) / 8
	size := templateHeaderSize + n*2 + maskBytes
	buf := make([]byte, size)
	off := 0
	copy(buf[off:], templateMagic[:])
	off += 4
	buf[off] = byte(t.Machine)
	off++
	buf[off] = byte(t.Eye)
	off++
	copy(buf[off:], attID)
	off += 32
	binary.BigEndian.PutUint32(buf[off:], t.Version)
	off += 4
	binary.BigEndian.PutUint16(buf[off:], t.Width)
	off += 2
	binary.BigEndian.PutUint16(buf[off:], t.Height)
	off += 2
	binary.BigEndian.PutUint16(buf[off:], t.Threshold)
	off += 2
	binary.BigEndian.PutUint64(buf[off:], math.Float64bits(t.Coverage))
	off += 8
	if t.Passed {
		buf[off] = 1
	}
	off++
	for i, v := range t.Z {
		binary.BigEndian.PutUint16(buf[off+i*2:], v)
	}
	off += n * 2
	for i, ok := range t.Mask {
		if ok {
			buf[off+i/8] |= 1 << uint(7-i%8)
		}
	}
	return buf, nil
}

// DecodeTemplate parses a blob previously produced by Template.Encode.
func DecodeTemplate(raw []byte) (*Template, error) {
	if len(raw) < templateHeaderSize {
		return nil, fmt.Errorf("%w: blob too short", ErrBadBlob)
	}
	var magic [4]byte
	copy(magic[:], raw[:4])
	if magic != templateMagic {
		return nil, fmt.Errorf("%w: bad magic", ErrBadBlob)
	}
	machine := Machine(raw[4])
	eye := Eye(raw[5])
	if !eye.Valid() || (machine != Host && machine != RightMachine) {
		return nil, fmt.Errorf("%w: bad machine/eye tag", ErrBadBlob)
	}
	off := 6
	attID := hex.EncodeToString(raw[off : off+32])
	off += 32
	t := &Template{
		Machine: machine,
		Eye:     eye,
		AttID:   attID,
		Version: binary.BigEndian.Uint32(raw[off:]),
	}
	off += 4
	t.Width = binary.BigEndian.Uint16(raw[off:])
	off += 2
	t.Height = binary.BigEndian.Uint16(raw[off:])
	off += 2
	t.Threshold = binary.BigEndian.Uint16(raw[off:])
	off += 2
	t.Coverage = math.Float64frombits(binary.BigEndian.Uint64(raw[off:]))
	off += 8
	t.Passed = raw[off] == 1
	off++
	n := int(t.Width) * int(t.Height)
	if len(raw) != templateHeaderSize+n*2+(n+7)/8 {
		return nil, fmt.Errorf("%w: size mismatch", ErrBadBlob)
	}
	t.Z = make([]uint16, n)
	for i := range t.Z {
		t.Z[i] = binary.BigEndian.Uint16(raw[off:])
		off += 2
	}
	t.Mask = make([]bool, n)
	for i := range t.Mask {
		t.Mask[i] = raw[off+i/8]&(1<<uint(7-i%8)) != 0
	}
	return t, nil
}
