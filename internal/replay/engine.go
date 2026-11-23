package replay

import (
	"crypto/sha256"
	"fmt"
	"sort"
)

type eyeTracker struct {
	latest  *acceptedSample
	version int
	lost    bool
	lostAt  int64
}

type engine struct {
	scenario Scenario
	frames   []FramePlan
	inputs   []InputRecord
	targets  []int
	order    []int
	seen     map[Eye]map[int64]int
	left     eyeTracker
	right    eyeTracker

	matrices  []MatrixRecord
	current   *MatrixRecord
	timeline  Timelines
	records   []FrameRecord
	resources []ResourceRecord
	reads     []ResourceReadRecord
	images    map[string][]byte

	nextSample int
	nextRead   int
}

func Run(scenario Scenario) (*Replay, error) {
	if scenario.SchemaVersion == 0 {
		scenario.SchemaVersion = SchemaVersion
	}
	if err := scenario.Validate(); err != nil {
		return nil, err
	}

	e := &engine{
		scenario: scenario,
		frames:   planFrames(scenario),
		seen:     map[Eye]map[int64]int{EyeLeft: {}, EyeRight: {}},
		images:   make(map[string][]byte),
	}
	e.indexInputs()

	for _, frame := range e.frames {
		submit := e.makeSubmitEvent(frame)
		e.timeline.Submit = append(e.timeline.Submit, submit)

		deadline := e.latch(frame)
		e.timeline.Deadline = append(e.timeline.Deadline, deadline)

		record := FrameRecord{
			Index:                frame.Index,
			ID:                   frame.ID,
			SubmitNs:             frame.SubmitNs,
			DeadlineNs:           frame.DeadlineNs,
			CompositeNs:          frame.Composite,
			ScanoutNs:            frame.ScanoutNs,
			UsedMatrixVersion:    deadline.FinalMatrixVersion,
			UsedLeftPoseVersion:  deadline.LeftPoseVersion,
			UsedRightPoseVersion: deadline.RightPoseVersion,
			UsedLeftSampleID:     deadline.LeftSampleID,
			UsedRightSampleID:    deadline.RightSampleID,
			MatrixReplaced:       deadline.Replaced,
			Resolution:           deadline.Resolution,
			TrackingState:        deadline.TrackingState,
		}

		composite := e.makeComposite(frame, &record, len(e.records))
		e.timeline.Composite = append(e.timeline.Composite, composite)
		record.OldResourceReadIDs = append([]string(nil), composite.OldResourceReadIDs...)

		image, err := renderFramePNG(e.current)
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", frame.ID, err)
		}
		sum := sha256.Sum256(image)
		record.ImagePath = fmt.Sprintf("frames/%s.png", frame.ID)
		record.ImageSHA256 = fmt.Sprintf("%x", sum[:])
		e.images[frame.ID] = image

		scanout := e.makeScanout(frame, &record)
		e.timeline.Scanout = append(e.timeline.Scanout, scanout)
		record.ColorResourceID = scanout.ResourceIDs[0]
		record.DepthResourceID = scanout.ResourceIDs[1]
		e.records = append(e.records, record)
	}

	e.markUnconsumedInputs()
	result := &Replay{
		SchemaVersion: SchemaVersion,
		Scenario:      scenario,
		Inputs:        e.inputs,
		Matrices:      e.matrices,
		Frames:        e.records,
		Resources:     e.resources,
		ResourceReads: e.reads,
		Timelines:     e.timeline,
		Images:        e.images,
	}
	result.Summary = e.summary()
	result.Checks = Validate(result)
	return result, nil
}

func (e *engine) indexInputs() {
	e.inputs = make([]InputRecord, len(e.scenario.Events))
	e.targets = make([]int, len(e.scenario.Events))
	e.order = make([]int, len(e.scenario.Events))
	for i, event := range e.scenario.Events {
		e.order[i] = i
		target := sort.Search(len(e.frames), func(j int) bool {
			return e.frames[j].DeadlineNs >= event.AtNs
		})
		e.targets[i] = target

		record := InputRecord{
			InputIndex:  i,
			EventID:     fmt.Sprintf("E%04d", i+1),
			Kind:        event.Kind,
			Eye:         event.Eye,
			ReceivedNs:  event.AtNs,
			TimestampNs: event.TimestampNs,
			YawDeg:      event.YawDeg,
			PitchDeg:    event.PitchDeg,
			RollDeg:     event.RollDeg,
			Status:      "pending",
			Note:        event.Note,
		}
		if target < len(e.frames) {
			record.TargetFrameID = e.frames[target].ID
		} else {
			record.Status = "after_final_deadline"
		}
		if open, ok := openFrameAt(e.frames, event.AtNs); ok && event.AtNs > e.frames[open].DeadlineNs {
			record.QueuedFromFrameID = e.frames[open].ID
		}
		e.inputs[i] = record
	}
	sort.SliceStable(e.order, func(i, j int) bool {
		left := e.scenario.Events[e.order[i]]
		right := e.scenario.Events[e.order[j]]
		return left.AtNs < right.AtNs
	})
}

func openFrameAt(frames []FramePlan, atNs int64) (int, bool) {
	index := sort.Search(len(frames), func(i int) bool {
		return frames[i].SubmitNs > atNs
	}) - 1
	if index < 0 || atNs >= frames[index].ScanoutNs {
		return 0, false
	}
	return index, true
}

func (e *engine) makeSubmitEvent(frame FramePlan) SubmitEvent {
	event := SubmitEvent{
		EventID: fmt.Sprintf("%s-SUBMIT", frame.ID),
		TimeNs:  frame.SubmitNs,
		FrameID: frame.ID,
	}
	if e.current != nil {
		event.ProvisionalMatrixVersion = e.current.Version
		event.ProvisionalLeftPoseVersion = e.current.LeftPoseVersion
		event.ProvisionalRightPoseVersion = e.current.RightPoseVersion
	}
	for _, i := range e.order {
		if e.targets[i] == frame.Index && e.inputs[i].ReceivedNs <= frame.SubmitNs {
			event.PendingEventIDs = append(event.PendingEventIDs, e.inputs[i].EventID)
		}
	}
	return event
}

func (e *engine) makeComposite(frame FramePlan, record *FrameRecord, priorIndex int) CompositeEvent {
	event := CompositeEvent{
		EventID:          fmt.Sprintf("%s-COMPOSITE", frame.ID),
		TimeNs:           frame.Composite,
		FrameID:          frame.ID,
		MatrixVersion:    record.UsedMatrixVersion,
		LeftPoseVersion:  record.UsedLeftPoseVersion,
		RightPoseVersion: record.UsedRightPoseVersion,
		Frozen:           true,
	}
	if !record.MatrixReplaced || priorIndex <= 0 {
		return event
	}
	prior := e.records[priorIndex-1]
	for _, resourceID := range []string{prior.ColorResourceID, prior.DepthResourceID} {
		resource, ok := e.resourceByID(resourceID)
		if !ok {
			continue
		}
		e.nextRead++
		read := ResourceReadRecord{
			ID:                    fmt.Sprintf("RR%04d", e.nextRead),
			AtNs:                  frame.Composite,
			ConsumerFrameID:       frame.ID,
			ConsumerMatrixVersion: record.UsedMatrixVersion,
			ResourceID:            resource.ID,
			ResourceVersion:       resource.Version,
			ProducerFrameID:       resource.ProducerFrameID,
			ProducerMatrixVersion: resource.ProducerMatrixVersion,
			Purpose:               "late-latch reprojection",
		}
		e.reads = append(e.reads, read)
		event.OldResourceReadIDs = append(event.OldResourceReadIDs, read.ID)
	}
	return event
}

func (e *engine) makeScanout(frame FramePlan, record *FrameRecord) ScanoutEvent {
	colorID := "color/" + frame.ID
	depthID := "depth/" + frame.ID
	version := frame.Index + 1
	e.resources = append(e.resources,
		ResourceRecord{
			ID: colorID, Version: version, Kind: "color",
			ProducerFrameID: frame.ID, ProducerMatrixVersion: record.UsedMatrixVersion,
			CreatedAtNs: frame.ScanoutNs, SHA256: record.ImageSHA256,
		},
		ResourceRecord{
			ID: depthID, Version: version, Kind: "depth",
			ProducerFrameID: frame.ID, ProducerMatrixVersion: record.UsedMatrixVersion,
			CreatedAtNs: frame.ScanoutNs,
		},
	)
	return ScanoutEvent{
		EventID:          fmt.Sprintf("%s-SCANOUT", frame.ID),
		TimeNs:           frame.ScanoutNs,
		FrameID:          frame.ID,
		MatrixVersion:    record.UsedMatrixVersion,
		LeftPoseVersion:  record.UsedLeftPoseVersion,
		RightPoseVersion: record.UsedRightPoseVersion,
		ImagePath:        record.ImagePath,
		ImageSHA256:      record.ImageSHA256,
		ResourceIDs:      []string{colorID, depthID},
	}
}

func (e *engine) resourceByID(id string) (ResourceRecord, bool) {
	for _, resource := range e.resources {
		if resource.ID == id {
			return resource, true
		}
	}
	return ResourceRecord{}, false
}

func (e *engine) markUnconsumedInputs() {
	for i, target := range e.targets {
		if target < len(e.frames) || e.inputs[i].Status != "pending" {
			continue
		}
		e.inputs[i].Status = "after_final_deadline"
	}
}

func (e *engine) summary() Summary {
	summary := Summary{
		FrameCount:       len(e.records),
		MatricesCreated:  len(e.matrices),
		OldResourceReads: len(e.reads),
	}
	for _, record := range e.records {
		if record.MatrixReplaced {
			summary.FramesReplaced++
		}
		if record.Resolution == "held_tracking_lost" {
			summary.HeldTrackingFrames++
		}
	}
	for _, input := range e.inputs {
		if input.TargetFrameID != "" && input.QueuedFromFrameID != "" {
			summary.LateQueuedEvents++
		}
		switch input.Status {
		case "duplicate":
			summary.DuplicateEvents++
		case "out_of_order":
			summary.OutOfOrderEvents++
		case "tracking_lost":
			summary.TrackingLossEvents++
		}
	}
	return summary
}
