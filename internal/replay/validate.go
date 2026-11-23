package replay

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

func Validate(result *Replay) CheckResult {
	check := CheckResult{OK: true}
	add := func(format string, args ...any) {
		check.OK = false
		check.Errors = append(check.Errors, fmt.Sprintf(format, args...))
	}
	if result == nil {
		return CheckResult{OK: false, Errors: []string{"nil replay"}}
	}

	inputs := indexBy(len(result.Inputs), func(i int) string { return result.Inputs[i].EventID })
	frames := indexBy(len(result.Frames), func(i int) string { return result.Frames[i].ID })
	matrices := make(map[int]MatrixRecord, len(result.Matrices))
	for _, matrix := range result.Matrices {
		if _, exists := matrices[matrix.Version]; exists {
			add("duplicate matrix version M%d", matrix.Version)
		}
		matrices[matrix.Version] = matrix
	}
	resources := make(map[string]ResourceRecord, len(result.Resources))
	for _, resource := range result.Resources {
		if _, exists := resources[resource.ID]; exists {
			add("duplicate resource %s", resource.ID)
		}
		resources[resource.ID] = resource
	}
	reads := make(map[string]ResourceReadRecord, len(result.ResourceReads))
	for _, read := range result.ResourceReads {
		if _, exists := reads[read.ID]; exists {
			add("duplicate resource read %s", read.ID)
		}
		reads[read.ID] = read
	}

	if len(result.Timelines.Submit) != len(result.Frames) ||
		len(result.Timelines.Deadline) != len(result.Frames) ||
		len(result.Timelines.Composite) != len(result.Frames) ||
		len(result.Timelines.Scanout) != len(result.Frames) {
		add("each frame must have exactly one event on all four timelines")
	}

	validateInputs(result.Inputs, inputs, add)
	validateMatrices(result, matrices, add)
	validateFrames(result, frames, matrices, resources, reads, add)
	validateResources(result, frames, matrices, resources, add)
	return check
}

func validateInputs(inputs []InputRecord, byID map[string]int, add func(string, ...any)) {
	for i := range inputs {
		input := &inputs[i]
		if input.TargetFrameID != "" {
			if input.Status == "pending" || input.Status == "after_final_deadline" {
				add("input %s has target %s but status %q", input.EventID, input.TargetFrameID, input.Status)
			}
		}
		if input.Status == "accepted" {
			if input.SampleID == "" || input.PoseVersion <= 0 {
				add("accepted input %s has no sample/pose version", input.EventID)
			}
		} else if input.SampleID != "" {
			add("non-accepted input %s unexpectedly has sample %s", input.EventID, input.SampleID)
		}
		if input.Status == "duplicate" {
			firstIndex, ok := byID[input.DuplicateOf]
			if !ok {
				add("input %s references unknown duplicate %s", input.EventID, input.DuplicateOf)
			} else if firstIndex >= i || inputs[firstIndex].Eye != input.Eye || inputs[firstIndex].TimestampNs != input.TimestampNs {
				add("input %s has invalid duplicate reference %s", input.EventID, input.DuplicateOf)
			}
		}
		if input.QueuedFromFrameID != "" && input.TargetFrameID == "" {
			add("input %s is queued from %s without a next target frame", input.EventID, input.QueuedFromFrameID)
		}
	}
}

func validateMatrices(result *Replay, matrices map[int]MatrixRecord, add func(string, ...any)) {
	samples := make(map[string]InputRecord)
	for _, input := range result.Inputs {
		if input.SampleID != "" {
			samples[input.SampleID] = input
		}
	}
	for i, matrix := range result.Matrices {
		if matrix.Version != i+1 {
			add("matrix version M%d is not sequential at index %d", matrix.Version, i)
		}
		if matrix.Digest != matrixDigest(matrix) {
			add("M%d digest does not match matrix values", matrix.Version)
		}
		left, leftOK := samples[matrix.LeftSampleID]
		right, rightOK := samples[matrix.RightSampleID]
		if !leftOK || left.Eye != EyeLeft {
			add("M%d references invalid left sample %s", matrix.Version, matrix.LeftSampleID)
		} else if left.PoseVersion != matrix.LeftPoseVersion {
			add("M%d left pose version does not match %s", matrix.Version, matrix.LeftSampleID)
		} else if left.ReceivedNs > matrix.CreatedAtNs {
			add("M%d left sample arrived after matrix creation", matrix.Version)
		}
		if !rightOK || right.Eye != EyeRight {
			add("M%d references invalid right sample %s", matrix.Version, matrix.RightSampleID)
		} else if right.PoseVersion != matrix.RightPoseVersion {
			add("M%d right pose version does not match %s", matrix.Version, matrix.RightSampleID)
		} else if right.ReceivedNs > matrix.CreatedAtNs {
			add("M%d right sample arrived after matrix creation", matrix.Version)
		}
		createdIndex, createdOK := resultFramesIndex(result, matrix.CreatedFrameID)
		if !createdOK {
			add("M%d references unknown creation frame %s", matrix.Version, matrix.CreatedFrameID)
		} else if result.Frames[createdIndex].DeadlineNs != matrix.CreatedAtNs {
			add("M%d creation time disagrees with %s deadline", matrix.Version, matrix.CreatedFrameID)
		}
	}
}

func resultFramesIndex(result *Replay, frameID string) (int, bool) {
	for i, frame := range result.Frames {
		if frame.ID == frameID {
			return i, true
		}
	}
	return 0, false
}

func validateFrames(
	result *Replay,
	frames map[string]int,
	matrices map[int]MatrixRecord,
	resources map[string]ResourceRecord,
	reads map[string]ResourceReadRecord,
	add func(string, ...any),
) {
	for i, frame := range result.Frames {
		if frame.Index != i {
			add("frame %s has index %d at position %d", frame.ID, frame.Index, i)
		}
		switch frame.Resolution {
		case "latched_initial_pose", "latched_new_pose", "held_same_pose",
			"held_tracking_lost", "held_incomplete_pose", "held_incomplete_update", "no_pose":
		default:
			add("frame %s has unknown resolution %q", frame.ID, frame.Resolution)
		}
		if frame.Index == 0 && frame.MatrixReplaced {
			add("initial frame %s cannot replace an existing matrix", frame.ID)
		}
		if len(result.Timelines.Submit) != len(result.Frames) ||
			len(result.Timelines.Deadline) != len(result.Frames) ||
			len(result.Timelines.Composite) != len(result.Frames) ||
			len(result.Timelines.Scanout) != len(result.Frames) {
			continue
		}
		submit := result.Timelines.Submit[i]
		deadline := result.Timelines.Deadline[i]
		composite := result.Timelines.Composite[i]
		scanout := result.Timelines.Scanout[i]
		if submit.FrameID != frame.ID || deadline.FrameID != frame.ID ||
			composite.FrameID != frame.ID || scanout.FrameID != frame.ID {
			add("timeline frame IDs disagree for %s", frame.ID)
			continue
		}
		if deadline.FinalMatrixVersion != frame.UsedMatrixVersion ||
			composite.MatrixVersion != frame.UsedMatrixVersion ||
			scanout.MatrixVersion != frame.UsedMatrixVersion {
			add("frame %s has inconsistent matrix versions across timeline", frame.ID)
		}
		if deadline.Replaced != frame.MatrixReplaced {
			add("frame %s replacement flag disagrees", frame.ID)
		}
		if scanout.ImagePath != frame.ImagePath || scanout.ImageSHA256 != frame.ImageSHA256 {
			add("frame %s image metadata disagrees with scanout", frame.ID)
		}
		if frame.UsedMatrixVersion == 0 {
			if frame.UsedLeftPoseVersion != 0 || frame.UsedRightPoseVersion != 0 {
				add("frame %s has pose versions but no matrix", frame.ID)
			}
		} else {
			matrix, ok := matrices[frame.UsedMatrixVersion]
			if !ok {
				add("frame %s references missing M%d", frame.ID, frame.UsedMatrixVersion)
			} else if matrix.LeftSampleID != frame.UsedLeftSampleID ||
				matrix.RightSampleID != frame.UsedRightSampleID ||
				matrix.LeftPoseVersion != frame.UsedLeftPoseVersion ||
				matrix.RightPoseVersion != frame.UsedRightPoseVersion {
				add("frame %s pose provenance disagrees with M%d", frame.ID, matrix.Version)
			}
		}
		if len(frame.ImageSHA256) != 64 {
			add("frame %s has invalid image digest", frame.ID)
		}
		if image, ok := result.Images[frame.ID]; ok {
			sum := sha256.Sum256(image)
			if fmt.Sprintf("%x", sum[:]) != frame.ImageSHA256 {
				add("frame %s image bytes disagree with digest", frame.ID)
			}
		}
		validateFrameResources(frame, resources, add)
		validateFrameReads(frame, composite, frames, resources, reads, add)
	}
}

func validateFrameResources(frame FrameRecord, resources map[string]ResourceRecord, add func(string, ...any)) {
	for _, id := range []string{frame.ColorResourceID, frame.DepthResourceID} {
		resource, ok := resources[id]
		if !ok {
			add("frame %s references missing resource %s", frame.ID, id)
			continue
		}
		if resource.ProducerFrameID != frame.ID || resource.ProducerMatrixVersion != frame.UsedMatrixVersion {
			add("frame %s resource %s has wrong producer provenance", frame.ID, id)
		}
	}
	if colorResource, ok := resources[frame.ColorResourceID]; ok && colorResource.SHA256 != frame.ImageSHA256 {
		add("frame %s color resource digest disagrees with image", frame.ID)
	}
}

func validateFrameReads(
	frame FrameRecord,
	composite CompositeEvent,
	frames map[string]int,
	resources map[string]ResourceRecord,
	reads map[string]ResourceReadRecord,
	add func(string, ...any),
) {
	if frame.Index == 0 || !frame.MatrixReplaced {
		if len(composite.OldResourceReadIDs) != 0 {
			add("frame %s reads old resources without a matrix replacement", frame.ID)
		}
		return
	}
	if len(composite.OldResourceReadIDs) != 2 {
		add("frame %s replacement should read prior color and depth resources", frame.ID)
	}
	for _, readID := range composite.OldResourceReadIDs {
		read, ok := reads[readID]
		if !ok {
			add("frame %s references missing read %s", frame.ID, readID)
			continue
		}
		if read.ConsumerFrameID != frame.ID || read.ConsumerMatrixVersion != frame.UsedMatrixVersion {
			add("read %s has wrong consumer provenance", readID)
		}
		resource, ok := resources[read.ResourceID]
		if !ok {
			add("read %s references missing resource %s", readID, read.ResourceID)
			continue
		}
		if read.ResourceVersion != resource.Version ||
			read.ProducerFrameID != resource.ProducerFrameID ||
			read.ProducerMatrixVersion != resource.ProducerMatrixVersion {
			add("read %s metadata disagrees with %s", readID, resource.ID)
		}
		if resource.CreatedAtNs > read.AtNs {
			add("read %s predates resource %s", readID, resource.ID)
		}
		if producerIndex, ok := frames[resource.ProducerFrameID]; ok {
			if producerIndex >= frame.Index {
				add("read %s does not reference an older frame", readID)
			}
		} else {
			add("read %s references unknown producer frame %s", readID, resource.ProducerFrameID)
		}
	}
}

func validateResources(
	result *Replay,
	frames map[string]int,
	matrices map[int]MatrixRecord,
	resources map[string]ResourceRecord,
	add func(string, ...any),
) {
	for _, resource := range result.Resources {
		producerIndex, ok := frames[resource.ProducerFrameID]
		if !ok {
			add("resource %s references unknown producer %s", resource.ID, resource.ProducerFrameID)
			continue
		}
		producer := result.Frames[producerIndex]
		if resource.ProducerMatrixVersion != producer.UsedMatrixVersion {
			add("resource %s producer matrix disagrees with frame", resource.ID)
		}
		if resource.ProducerMatrixVersion != 0 {
			if _, ok := matrices[resource.ProducerMatrixVersion]; !ok {
				add("resource %s references missing M%d", resource.ID, resource.ProducerMatrixVersion)
			}
		}
	}
	referenced := make(map[string]bool)
	for _, composite := range result.Timelines.Composite {
		for _, readID := range composite.OldResourceReadIDs {
			referenced[readID] = true
		}
	}
	for _, read := range result.ResourceReads {
		if !strings.HasPrefix(read.ID, "RR") {
			add("resource read %s has invalid ID", read.ID)
		}
		if !referenced[read.ID] {
			add("read %s is not attached to a composite event", read.ID)
		}
		resource, ok := resources[read.ResourceID]
		if !ok {
			add("read %s references unknown resource %s", read.ID, read.ResourceID)
			continue
		}
		if read.ResourceVersion != resource.Version ||
			read.ProducerFrameID != resource.ProducerFrameID ||
			read.ProducerMatrixVersion != resource.ProducerMatrixVersion {
			add("read %s provenance disagrees with resource %s", read.ID, resource.ID)
		}
		producerIndex, producerOK := frames[read.ProducerFrameID]
		consumerIndex, consumerOK := frames[read.ConsumerFrameID]
		if !producerOK || !consumerOK {
			add("read %s has an unknown producer or consumer frame", read.ID)
			continue
		}
		if producerIndex >= consumerIndex {
			add("read %s does not reference an older producer frame", read.ID)
		}
		if result.Frames[producerIndex].UsedMatrixVersion != read.ProducerMatrixVersion {
			add("read %s producer matrix disagrees with producer frame", read.ID)
		}
		if result.Frames[consumerIndex].UsedMatrixVersion != read.ConsumerMatrixVersion {
			add("read %s consumer matrix disagrees with consumer frame", read.ID)
		}
	}
}

func indexBy(length int, key func(int) string) map[string]int {
	index := make(map[string]int, length)
	for i := 0; i < length; i++ {
		index[key(i)] = i
	}
	return index
}
