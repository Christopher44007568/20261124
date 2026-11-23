package depthlife

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

// Binary layout of a depth attachment (all integers big-endian):
//
//	[0:4]   magic "DLAF"
//	[4:8]   format version (uint32)
//	[8:10]  width  (uint16)
//	[10:12] height (uint16)
//	[12:..] width*height depth samples (uint16 each)
var attachmentMagic = [4]byte{'D', 'L', 'A', 'F'}

const attachmentHeaderSize = 12

// Attachment is a raw depth attachment plus its decoded header. The same
// attachment may be loaded for both eyes; Raw is the source of identity.
type Attachment struct {
	ID      string
	Version uint32
	Width   uint16
	Height  uint16
	Raw     []byte
}

// DecodeAttachment validates the payload and extracts its header. Sample
// bytes stay untouched until the attachment is parsed for a specific eye.
func DecodeAttachment(raw []byte) (Attachment, error) {
	if len(raw) < attachmentHeaderSize {
		return Attachment{}, fmt.Errorf("%w: payload shorter than header", ErrBadBlob)
	}
	var magic [4]byte
	copy(magic[:], raw[:4])
	if magic != attachmentMagic {
		return Attachment{}, fmt.Errorf("%w: bad magic", ErrBadBlob)
	}
	version := binary.BigEndian.Uint32(raw[4:8])
	width := binary.BigEndian.Uint16(raw[8:10])
	height := binary.BigEndian.Uint16(raw[10:12])
	if width == 0 || height == 0 {
		return Attachment{}, fmt.Errorf("%w: zero dimensions", ErrBadBlob)
	}
	want := attachmentHeaderSize + int(width)*int(height)*2
	if len(raw) != want {
		return Attachment{}, fmt.Errorf("%w: size %d, want %d", ErrBadBlob, len(raw), want)
	}
	sum := sha256.Sum256(raw)
	return Attachment{
		ID:      hex.EncodeToString(sum[:]),
		Version: version,
		Width:   width,
		Height:  height,
		Raw:     append([]byte(nil), raw...),
	}, nil
}

// EncodeAttachment builds a payload for the given format version and samples.
func EncodeAttachment(version uint32, width, height uint16, samples []uint16) []byte {
	raw := make([]byte, attachmentHeaderSize+len(samples)*2)
	copy(raw[:4], attachmentMagic[:])
	binary.BigEndian.PutUint32(raw[4:8], version)
	binary.BigEndian.PutUint16(raw[8:10], width)
	binary.BigEndian.PutUint16(raw[10:12], height)
	for i, v := range samples {
		binary.BigEndian.PutUint16(raw[attachmentHeaderSize+i*2:], v)
	}
	return raw
}

// DepthResult is the eye-specific output of parsing an attachment.
type DepthResult struct {
	Eye     Eye
	AttID   string
	Version uint32
	Width   uint16
	Height  uint16
	Z       []uint16
	MeanZ   uint32
	MinZ    uint16
	MaxZ    uint16
}
