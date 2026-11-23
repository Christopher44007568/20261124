package replay

import (
	"fmt"
)

const SchemaVersion = 1

type Eye string

const (
	EyeLeft  Eye = "left"
	EyeRight Eye = "right"
)

func (e Eye) Valid() bool {
	return e == EyeLeft || e == EyeRight
}

type InputEvent struct {
	AtNs        int64   `json:"atNs"`
	Kind        string  `json:"kind"`
	Eye         Eye     `json:"eye"`
	TimestampNs int64   `json:"timestampNs,omitempty"`
	YawDeg      float64 `json:"yawDeg,omitempty"`
	PitchDeg    float64 `json:"pitchDeg,omitempty"`
	RollDeg     float64 `json:"rollDeg,omitempty"`
	Note        string  `json:"note,omitempty"`
}

type Scenario struct {
	SchemaVersion     int          `json:"schemaVersion"`
	Name              string       `json:"name"`
	Description       string       `json:"description"`
	FrameCount        int          `json:"frameCount"`
	FirstSubmitNs     int64        `json:"firstSubmitNs"`
	FramePeriodNs     int64        `json:"framePeriodNs"`
	DeadlineOffsetNs  int64        `json:"deadlineOffsetNs"`
	CompositeOffsetNs int64        `json:"compositeOffsetNs"`
	ScanoutOffsetNs   int64        `json:"scanoutOffsetNs"`
	Events            []InputEvent `json:"events"`
}

func (s Scenario) Validate() error {
	if s.SchemaVersion != 0 && s.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported schemaVersion %d", s.SchemaVersion)
	}
	if s.Name == "" {
		return fmt.Errorf("name is required")
	}
	if s.FrameCount <= 0 || s.FrameCount > 100000 {
		return fmt.Errorf("frameCount must be between 1 and 100000")
	}
	if s.FramePeriodNs <= 0 {
		return fmt.Errorf("framePeriodNs must be positive")
	}
	if s.DeadlineOffsetNs < 0 || s.DeadlineOffsetNs > s.CompositeOffsetNs {
		return fmt.Errorf("deadlineOffsetNs must be non-negative and no later than compositeOffsetNs")
	}
	if s.CompositeOffsetNs > s.ScanoutOffsetNs {
		return fmt.Errorf("compositeOffsetNs must be no later than scanoutOffsetNs")
	}
	if s.ScanoutOffsetNs > s.FramePeriodNs {
		return fmt.Errorf("scanoutOffsetNs must not exceed framePeriodNs")
	}
	for i, event := range s.Events {
		if !event.Eye.Valid() {
			return fmt.Errorf("events[%d]: eye must be left or right", i)
		}
		switch event.Kind {
		case "pose":
		case "tracking_lost":
		default:
			return fmt.Errorf("events[%d]: unsupported kind %q", i, event.Kind)
		}
	}
	return nil
}

type FramePlan struct {
	Index      int
	ID         string
	SubmitNs   int64
	DeadlineNs int64
	Composite  int64
	ScanoutNs  int64
}

func planFrames(s Scenario) []FramePlan {
	frames := make([]FramePlan, s.FrameCount)
	for i := range frames {
		submit := s.FirstSubmitNs + int64(i)*s.FramePeriodNs
		frames[i] = FramePlan{
			Index:      i,
			ID:         fmt.Sprintf("F%04d", i+1),
			SubmitNs:   submit,
			DeadlineNs: submit + s.DeadlineOffsetNs,
			Composite:  submit + s.CompositeOffsetNs,
			ScanoutNs:  submit + s.ScanoutOffsetNs,
		}
	}
	return frames
}

type Matrix4 [16]float64

type MatrixRecord struct {
	Version          int     `json:"version"`
	Digest           string  `json:"digest"`
	CreatedFrameID   string  `json:"createdFrameId"`
	CreatedAtNs      int64   `json:"createdAtNs"`
	LeftSampleID     string  `json:"leftSampleId"`
	RightSampleID    string  `json:"rightSampleId"`
	LeftPoseVersion  int     `json:"leftPoseVersion"`
	RightPoseVersion int     `json:"rightPoseVersion"`
	LeftYawDeg       float64 `json:"leftYawDeg"`
	LeftPitchDeg     float64 `json:"leftPitchDeg"`
	LeftRollDeg      float64 `json:"leftRollDeg"`
	RightYawDeg      float64 `json:"rightYawDeg"`
	RightPitchDeg    float64 `json:"rightPitchDeg"`
	RightRollDeg     float64 `json:"rightRollDeg"`
	LeftEye          Matrix4 `json:"leftEye"`
	RightEye         Matrix4 `json:"rightEye"`
}

type InputRecord struct {
	InputIndex        int     `json:"inputIndex"`
	EventID           string  `json:"eventId"`
	Kind              string  `json:"kind"`
	Eye               Eye     `json:"eye"`
	ReceivedNs        int64   `json:"receivedNs"`
	TimestampNs       int64   `json:"timestampNs,omitempty"`
	YawDeg            float64 `json:"yawDeg,omitempty"`
	PitchDeg          float64 `json:"pitchDeg,omitempty"`
	RollDeg           float64 `json:"rollDeg,omitempty"`
	SampleID          string  `json:"sampleId,omitempty"`
	PoseVersion       int     `json:"poseVersion,omitempty"`
	Status            string  `json:"status"`
	TargetFrameID     string  `json:"targetFrameId,omitempty"`
	QueuedFromFrameID string  `json:"queuedFromFrameId,omitempty"`
	DuplicateOf       string  `json:"duplicateOf,omitempty"`
	Note              string  `json:"note,omitempty"`
}

type SubmitEvent struct {
	EventID                     string   `json:"eventId"`
	TimeNs                      int64    `json:"timeNs"`
	FrameID                     string   `json:"frameId"`
	ProvisionalMatrixVersion    int      `json:"provisionalMatrixVersion"`
	ProvisionalLeftPoseVersion  int      `json:"provisionalLeftPoseVersion"`
	ProvisionalRightPoseVersion int      `json:"provisionalRightPoseVersion"`
	PendingEventIDs             []string `json:"pendingEventIds"`
}

type DeadlineEvent struct {
	EventID            string   `json:"eventId"`
	TimeNs             int64    `json:"timeNs"`
	FrameID            string   `json:"frameId"`
	InputMatrixVersion int      `json:"inputMatrixVersion"`
	FinalMatrixVersion int      `json:"finalMatrixVersion"`
	Replaced           bool     `json:"replaced"`
	Resolution         string   `json:"resolution"`
	LeftSampleID       string   `json:"leftSampleId,omitempty"`
	RightSampleID      string   `json:"rightSampleId,omitempty"`
	LeftPoseVersion    int      `json:"leftPoseVersion"`
	RightPoseVersion   int      `json:"rightPoseVersion"`
	TrackingState      string   `json:"trackingState"`
	AcceptedEventIDs   []string `json:"acceptedEventIds"`
	IgnoredEventIDs    []string `json:"ignoredEventIds"`
	TrackingEventIDs   []string `json:"trackingEventIds"`
	LateQueuedEventIDs []string `json:"lateQueuedEventIds"`
}

type CompositeEvent struct {
	EventID            string   `json:"eventId"`
	TimeNs             int64    `json:"timeNs"`
	FrameID            string   `json:"frameId"`
	MatrixVersion      int      `json:"matrixVersion"`
	LeftPoseVersion    int      `json:"leftPoseVersion"`
	RightPoseVersion   int      `json:"rightPoseVersion"`
	Frozen             bool     `json:"frozen"`
	OldResourceReadIDs []string `json:"oldResourceReadIds"`
}

type ScanoutEvent struct {
	EventID          string   `json:"eventId"`
	TimeNs           int64    `json:"timeNs"`
	FrameID          string   `json:"frameId"`
	MatrixVersion    int      `json:"matrixVersion"`
	LeftPoseVersion  int      `json:"leftPoseVersion"`
	RightPoseVersion int      `json:"rightPoseVersion"`
	ImagePath        string   `json:"imagePath"`
	ImageSHA256      string   `json:"imageSha256"`
	ResourceIDs      []string `json:"resourceIds"`
}

type Timelines struct {
	Submit    []SubmitEvent    `json:"submit"`
	Deadline  []DeadlineEvent  `json:"deadline"`
	Composite []CompositeEvent `json:"composite"`
	Scanout   []ScanoutEvent   `json:"scanout"`
}

type FrameRecord struct {
	Index                int      `json:"index"`
	ID                   string   `json:"id"`
	SubmitNs             int64    `json:"submitNs"`
	DeadlineNs           int64    `json:"deadlineNs"`
	CompositeNs          int64    `json:"compositeNs"`
	ScanoutNs            int64    `json:"scanoutNs"`
	UsedMatrixVersion    int      `json:"usedMatrixVersion"`
	UsedLeftPoseVersion  int      `json:"usedLeftPoseVersion"`
	UsedRightPoseVersion int      `json:"usedRightPoseVersion"`
	UsedLeftSampleID     string   `json:"usedLeftSampleId,omitempty"`
	UsedRightSampleID    string   `json:"usedRightSampleId,omitempty"`
	MatrixReplaced       bool     `json:"matrixReplaced"`
	Resolution           string   `json:"resolution"`
	TrackingState        string   `json:"trackingState"`
	ImagePath            string   `json:"imagePath"`
	ImageSHA256          string   `json:"imageSha256"`
	ColorResourceID      string   `json:"colorResourceId"`
	DepthResourceID      string   `json:"depthResourceId"`
	OldResourceReadIDs   []string `json:"oldResourceReadIds"`
}

type ResourceRecord struct {
	ID                    string `json:"id"`
	Version               int    `json:"version"`
	Kind                  string `json:"kind"`
	ProducerFrameID       string `json:"producerFrameId"`
	ProducerMatrixVersion int    `json:"producerMatrixVersion"`
	CreatedAtNs           int64  `json:"createdAtNs"`
	SHA256                string `json:"sha256,omitempty"`
}

type ResourceReadRecord struct {
	ID                    string `json:"id"`
	AtNs                  int64  `json:"atNs"`
	ConsumerFrameID       string `json:"consumerFrameId"`
	ConsumerMatrixVersion int    `json:"consumerMatrixVersion"`
	ResourceID            string `json:"resourceId"`
	ResourceVersion       int    `json:"resourceVersion"`
	ProducerFrameID       string `json:"producerFrameId"`
	ProducerMatrixVersion int    `json:"producerMatrixVersion"`
	Purpose               string `json:"purpose"`
}

type AuditRow struct {
	FrameID             string
	SubmitNs            int64
	DeadlineNs          int64
	CompositeNs         int64
	ScanoutNs           int64
	MatrixVersion       int
	LeftPoseVersion     int
	RightPoseVersion    int
	MatrixReplaced      bool
	TrackingState       string
	Resolution          string
	ImagePath           string
	ImageSHA256         string
	OldResourceReadIDs  string
	OldProducerMatrices string
}

type Summary struct {
	FrameCount         int `json:"frameCount"`
	MatricesCreated    int `json:"matricesCreated"`
	FramesReplaced     int `json:"framesReplaced"`
	LateQueuedEvents   int `json:"lateQueuedEvents"`
	DuplicateEvents    int `json:"duplicateEvents"`
	OutOfOrderEvents   int `json:"outOfOrderEvents"`
	TrackingLossEvents int `json:"trackingLossEvents"`
	HeldTrackingFrames int `json:"heldTrackingFrames"`
	OldResourceReads   int `json:"oldResourceReads"`
}

type CheckResult struct {
	OK     bool     `json:"ok"`
	Errors []string `json:"errors"`
}

type Replay struct {
	SchemaVersion int                  `json:"schemaVersion"`
	Scenario      Scenario             `json:"scenario"`
	Summary       Summary              `json:"summary"`
	Inputs        []InputRecord        `json:"inputs"`
	Matrices      []MatrixRecord       `json:"matrices"`
	Frames        []FrameRecord        `json:"frames"`
	Resources     []ResourceRecord     `json:"resources"`
	ResourceReads []ResourceReadRecord `json:"resourceReads"`
	Timelines     Timelines            `json:"timelines"`
	Checks        CheckResult          `json:"checks"`
	Images        map[string][]byte    `json:"-"`
}
