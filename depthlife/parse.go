package depthlife

import "math"

// eyeTagged is implemented by every eye-scoped artifact. The manager uses it
// to prove an artifact really belongs to the eye that is asking for it, so a
// result cached for the left eye can never silently serve the right eye.
type eyeTagged interface {
	TaggedEye() Eye
	TaggedAttachment() string
}

func (d *DepthResult) TaggedEye() Eye           { return d.Eye }
func (d *DepthResult) TaggedAttachment() string { return d.AttID }

// parseDepth turns an attachment into an eye-specific depth result. The raw
// bytes are shared, but the resulting object is independently owned by the
// eye slot.
func parseDepth(a Attachment, eye Eye) *DepthResult {
	n := int(a.Width) * int(a.Height)
	z := make([]uint16, n)
	var sum uint64
	minZ := uint16(math.MaxUint16)
	maxZ := uint16(0)
	for i := range z {
		v := uint16(a.Raw[attachmentHeaderSize+i*2])<<8 |
			uint16(a.Raw[attachmentHeaderSize+i*2+1])
		z[i] = v
		sum += uint64(v)
		if v < minZ {
			minZ = v
		}
		if v > maxZ {
			maxZ = v
		}
	}
	mean := uint32(0)
	if n > 0 {
		mean = uint32(sum / uint64(n))
	}
	return &DepthResult{
		Eye:     eye,
		AttID:   a.ID,
		Version: a.Version,
		Width:   a.Width,
		Height:  a.Height,
		Z:       z,
		MeanZ:   mean,
		MinZ:    minZ,
		MaxZ:    maxZ,
	}
}

// MaskResult is the eye-specific output of applying a threshold mask.
type MaskResult struct {
	Eye       Eye
	AttID     string
	Threshold uint16
	Coverage  float64
	Passed    bool
	Mask      []bool
}

func (m *MaskResult) TaggedEye() Eye           { return m.Eye }
func (m *MaskResult) TaggedAttachment() string { return m.AttID }

// applyMask marks every sample at or above threshold as valid. A template
// passes only when valid coverage reaches minCoverage (0..1).
func applyMask(d *DepthResult, threshold uint16, minCoverage float64) *MaskResult {
	mask := make([]bool, len(d.Z))
	valid := 0
	for i, v := range d.Z {
		if v >= threshold {
			mask[i] = true
			valid++
		}
	}
	cov := 0.0
	if len(d.Z) > 0 {
		cov = float64(valid) / float64(len(d.Z))
	}
	return &MaskResult{
		Eye:       d.Eye,
		AttID:     d.AttID,
		Threshold: threshold,
		Coverage:  cov,
		Passed:    cov >= minCoverage,
		Mask:      mask,
	}
}
