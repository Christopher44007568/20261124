package timewarp

import "fmt"

// Stopped reports whether the pipeline permanently stopped after a clock
// regression.
func (p *Pipeline) Stopped() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.stopped
}

// Stats returns a snapshot of accepted-work counters.
func (p *Pipeline) Stats() Stats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.stats
}

// Records returns a copy of all warp audit entries in sequence order.
func (p *Pipeline) Records() []WarpRecord {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]WarpRecord, len(p.records))
	copy(out, p.records)
	return out
}

// WarpsForFrame returns how many actual reprojections consumed a source frame.
func (p *Pipeline) WarpsForFrame(frameID uint64) uint64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.warpsByFrame[frameID]
}

// Provenance returns the source-to-final image chain ending at imageID. The
// returned slice includes both endpoints.
func (p *Pipeline) Provenance(imageID string) ([]Image, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.provenanceLocked(imageID)
}

// SourceChain is an explicit alias for Provenance.
func (p *Pipeline) SourceChain(imageID string) ([]Image, error) {
	return p.Provenance(imageID)
}

func (p *Pipeline) provenanceLocked(imageID string) ([]Image, error) {
	reversed := make([]Image, 0, 8)
	seen := make(map[string]struct{})
	for imageID != "" {
		if _, duplicate := seen[imageID]; duplicate {
			return nil, fmt.Errorf("%w: at %q", ErrProvenanceCycle, imageID)
		}
		seen[imageID] = struct{}{}

		image, ok := p.images[imageID]
		if !ok {
			return nil, fmt.Errorf("%w: %q", ErrUnknownImage, imageID)
		}
		reversed = append(reversed, image)
		imageID = image.ParentID
	}

	for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
		reversed[left], reversed[right] = reversed[right], reversed[left]
	}
	return reversed, nil
}
