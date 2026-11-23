package timewarp

import (
	"fmt"
	"sync"
)

// Pipeline consumes rendered frames and poses in monotonic timestamp order and
// produces one reprojected image per display scanout.
//
// A source frame can feed several scanouts. Each actual reprojection consumes
// the previous intermediate image, creating a linked provenance chain. A
// duplicate frame ID is ignored: it does not replace the current source and
// does not increment accepted-work counters.
type Pipeline struct {
	mu sync.RWMutex

	highWater Timestamp
	clockSeen bool
	stopped   bool

	lastFrame *Frame
	lastPose  *Pose

	frameIDs     map[uint64]struct{}
	images       map[string]Image
	records      []WarpRecord
	warpsByFrame map[uint64]uint64
	nextWarp     uint64

	lastImageID          string
	lastImageSourceFrame uint64
	stats                Stats
}

// NewPipeline returns an empty pipeline.
func NewPipeline() *Pipeline {
	return &Pipeline{
		frameIDs:     make(map[uint64]struct{}),
		images:       make(map[string]Image),
		warpsByFrame: make(map[uint64]uint64),
	}
}

// SubmitFrame adds a rendered frame. It returns false for a duplicate frame ID.
// Duplicate frames are counted in Stats.DuplicateFrames but never become the
// active source another time.
func (p *Pipeline) SubmitFrame(frame Frame) (bool, error) {
	if err := frame.Validate(); err != nil {
		return false, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.observeLocked(frame.RenderTime); err != nil {
		return false, err
	}
	if _, exists := p.frameIDs[frame.ID]; exists {
		p.stats.DuplicateFrames++
		return false, nil
	}

	imageID := sourceImageID(frame.ID)
	p.frameIDs[frame.ID] = struct{}{}
	p.images[imageID] = Image{
		ID:            imageID,
		Kind:          ImageSource,
		SourceFrameID: frame.ID,
		SourceImageID: imageID,
		Pose:          frame.Pose,
		CreatedAt:     frame.RenderTime,
		ChainDepth:    1,
	}

	stored := frame
	p.lastFrame = &stored
	p.stats.FramesAccepted++
	return true, nil
}

// SubmitPose records the newest complete pose snapshot. Pose versions must be
// strictly increasing; stale or duplicate versions are ignored.
func (p *Pipeline) SubmitPose(pose Pose) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.observeLocked(pose.At); err != nil {
		return false, err
	}
	if p.lastPose != nil && pose.Version <= p.lastPose.Version {
		p.stats.DuplicatePoses++
		return false, nil
	}

	stored := pose
	p.lastPose = &stored
	p.stats.PosesAccepted++
	return true, nil
}

// Scanout reprojects the newest source frame to the newest pose known at this
// display timestamp. If no newer pose exists, the prior image is reused and no
// warp record or counter is added.
func (p *Pipeline) Scanout(at Timestamp) (Scanout, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.observeLocked(at); err != nil {
		return Scanout{}, err
	}
	p.stats.Scanouts++
	if p.lastFrame == nil {
		p.stats.NoFrameScanouts++
		return Scanout{}, ErrNoFrame
	}
	if p.lastPose == nil {
		return Scanout{}, ErrNoPose
	}

	frame := *p.lastFrame
	fromID, fromPose, reused := p.historyLocked(frame)
	target := *p.lastPose

	if target.Version <= fromPose.Version || sameOrientation(target, fromPose) {
		scanout, err := p.reuseLocked(at, frame, fromID)
		if err != nil {
			return Scanout{}, err
		}
		return scanout, nil
	}

	return p.warpLocked(at, frame, fromID, fromPose, target, reused)
}

func (p *Pipeline) historyLocked(frame Frame) (string, Pose, bool) {
	if p.lastImageID != "" && p.lastImageSourceFrame == frame.ID {
		image := p.images[p.lastImageID]
		return p.lastImageID, image.Pose, image.Kind == ImageIntermediate
	}
	id := sourceImageID(frame.ID)
	return id, frame.Pose, false
}

func (p *Pipeline) reuseLocked(at Timestamp, frame Frame, imageID string) (Scanout, error) {
	provenance, err := p.provenanceLocked(imageID)
	if err != nil {
		return Scanout{}, err
	}
	p.lastImageID = imageID
	p.lastImageSourceFrame = frame.ID
	return Scanout{
		At:            at,
		SourceFrameID: frame.ID,
		ImageID:       imageID,
		Provenance:    provenance,
	}, nil
}

func (p *Pipeline) warpLocked(
	at Timestamp,
	frame Frame,
	fromID string,
	fromPose Pose,
	target Pose,
	reused bool,
) (Scanout, error) {
	p.nextWarp++
	sequence := p.nextWarp
	outputID := intermediateImageID(sequence)
	depth := p.images[fromID].ChainDepth + 1

	output := Image{
		ID:            outputID,
		Kind:          ImageIntermediate,
		ParentID:      fromID,
		SourceFrameID: frame.ID,
		SourceImageID: sourceImageID(frame.ID),
		Pose:          target,
		CreatedAt:     at,
		WarpSequence:  sequence,
		ChainDepth:    depth,
	}
	record := WarpRecord{
		Sequence:               sequence,
		ScanoutAt:              at,
		SourceFrameID:          frame.ID,
		SourceFramePoseVersion: frame.Pose.Version,
		HistorySourceID:        fromID,
		HistoryPoseVersion:     fromPose.Version,
		IntermediateImageID:    outputID,
		TargetPoseVersion:      target.Version,
		FromPose:               fromPose,
		TargetPose:             target,
		DeltaYaw:               YawDelta(fromPose.Yaw, target.Yaw),
		FrameAge:               at - frame.RenderTime,
		ReusedFrame:            reused,
		ChainDepth:             depth,
	}

	p.images[outputID] = output
	p.records = append(p.records, record)
	p.warpsByFrame[frame.ID]++
	p.lastImageID = outputID
	p.lastImageSourceFrame = frame.ID
	p.stats.Warps++
	if reused {
		p.stats.ReusedFrameWarps++
	}

	provenance, err := p.provenanceLocked(outputID)
	if err != nil {
		return Scanout{}, err
	}
	recordCopy := record
	return Scanout{
		At:            at,
		SourceFrameID: frame.ID,
		ImageID:       outputID,
		Warped:        true,
		Record:        &recordCopy,
		Provenance:    provenance,
	}, nil
}

func (p *Pipeline) observeLocked(at Timestamp) error {
	if p.stopped {
		return fmt.Errorf("%w: %w", ErrWarpStopped, ErrClockWentBackwards)
	}
	if p.clockSeen && at < p.highWater {
		p.stopped = true
		p.stats.ClockStops++
		return fmt.Errorf("%w: previous=%dus received=%dus", ErrClockWentBackwards, p.highWater, at)
	}
	if !p.clockSeen || at > p.highWater {
		p.highWater = at
	}
	p.clockSeen = true
	return nil
}

func sourceImageID(frameID uint64) string {
	return fmt.Sprintf("src/%d", frameID)
}

func intermediateImageID(sequence uint64) string {
	return fmt.Sprintf("warp/%d", sequence)
}
