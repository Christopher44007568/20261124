package replay

import "fmt"

func (e *engine) latch(frame FramePlan) DeadlineEvent {
	event := DeadlineEvent{
		EventID: fmt.Sprintf("%s-DEADLINE", frame.ID),
		TimeNs:  frame.DeadlineNs,
		FrameID: frame.ID,
	}
	if e.current != nil {
		event.InputMatrixVersion = e.current.Version
	}

	for _, i := range e.order {
		if e.targets[i] != frame.Index {
			continue
		}
		eventID := e.inputs[i].EventID
		input := e.scenario.Events[i]
		switch input.Kind {
		case "tracking_lost":
			e.tracker(input.Eye).lost = true
			e.tracker(input.Eye).lostAt = input.AtNs
			e.inputs[i].Status = "tracking_lost"
			event.TrackingEventIDs = append(event.TrackingEventIDs, eventID)
		case "pose":
			e.applyPose(i, input, &event)
		}
	}

	left := e.availableSample(EyeLeft)
	right := e.availableSample(EyeRight)
	event.TrackingState = e.trackingState()
	event.Resolution = "held_same_pose"

	if left != nil && right != nil {
		leftChanged := e.current == nil || e.current.LeftSampleID != left.ID
		rightChanged := e.current == nil || e.current.RightSampleID != right.ID
		if leftChanged && rightChanged {
			initial := e.current == nil
			record := buildMatrixRecord(len(e.matrices)+1, frame, left, right)
			e.matrices = append(e.matrices, record)
			current := record
			e.current = &current
			if initial {
				event.Resolution = "latched_initial_pose"
			} else {
				event.Resolution = "latched_new_pose"
			}
		} else if leftChanged || rightChanged {
			event.Resolution = "held_incomplete_update"
		}
	} else if e.left.lost || e.right.lost {
		event.Resolution = "held_tracking_lost"
	} else if e.current == nil {
		event.Resolution = "no_pose"
	} else {
		event.Resolution = "held_incomplete_pose"
	}

	if e.current != nil {
		event.FinalMatrixVersion = e.current.Version
		event.LeftSampleID = e.current.LeftSampleID
		event.RightSampleID = e.current.RightSampleID
		event.LeftPoseVersion = e.current.LeftPoseVersion
		event.RightPoseVersion = e.current.RightPoseVersion
	}
	event.Replaced = event.InputMatrixVersion > 0 && event.FinalMatrixVersion != event.InputMatrixVersion

	for _, i := range e.order {
		if e.inputs[i].QueuedFromFrameID == frame.ID && e.targets[i] < len(e.frames) {
			event.LateQueuedEventIDs = append(event.LateQueuedEventIDs, e.inputs[i].EventID)
		}
	}
	return event
}

func (e *engine) applyPose(index int, input InputEvent, event *DeadlineEvent) {
	inputID := e.inputs[index].EventID
	tracker := e.tracker(input.Eye)

	if first, ok := e.seen[input.Eye][input.TimestampNs]; ok {
		e.inputs[index].Status = "duplicate"
		e.inputs[index].DuplicateOf = e.inputs[first].EventID
		event.IgnoredEventIDs = append(event.IgnoredEventIDs, inputID)
		return
	}
	e.seen[input.Eye][input.TimestampNs] = index

	if tracker.latest != nil && input.TimestampNs <= tracker.latest.TimestampNs {
		e.inputs[index].Status = "out_of_order"
		event.IgnoredEventIDs = append(event.IgnoredEventIDs, inputID)
		return
	}

	e.nextSample++
	tracker.version++
	tracker.latest = &acceptedSample{
		ID:          fmt.Sprintf("S%04d", e.nextSample),
		Eye:         input.Eye,
		Version:     tracker.version,
		TimestampNs: input.TimestampNs,
		ReceivedNs:  input.AtNs,
		YawDeg:      input.YawDeg,
		PitchDeg:    input.PitchDeg,
		RollDeg:     input.RollDeg,
		InputIndex:  index,
	}
	tracker.lost = false
	e.inputs[index].SampleID = tracker.latest.ID
	e.inputs[index].PoseVersion = tracker.latest.Version
	e.inputs[index].Status = "accepted"
	event.AcceptedEventIDs = append(event.AcceptedEventIDs, inputID)
}

func (e *engine) tracker(eye Eye) *eyeTracker {
	if eye == EyeLeft {
		return &e.left
	}
	return &e.right
}

func (e *engine) availableSample(eye Eye) *acceptedSample {
	tracker := e.tracker(eye)
	if tracker.lost {
		return nil
	}
	return tracker.latest
}

func (e *engine) trackingState() string {
	return fmt.Sprintf("left=%s,right=%s", eyeStateText(&e.left), eyeStateText(&e.right))
}

func eyeStateText(tracker *eyeTracker) string {
	if tracker.lost {
		return "lost"
	}
	if tracker.latest == nil {
		return "waiting"
	}
	return "tracking"
}
