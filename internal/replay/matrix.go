package replay

import (
	"crypto/sha256"
	"fmt"
	"math"
	"strings"
)

type acceptedSample struct {
	ID          string
	Eye         Eye
	Version     int
	TimestampNs int64
	ReceivedNs  int64
	YawDeg      float64
	PitchDeg    float64
	RollDeg     float64
	InputIndex  int
}

func buildMatrixRecord(version int, frame FramePlan, left, right *acceptedSample) MatrixRecord {
	record := MatrixRecord{
		Version:          version,
		CreatedFrameID:   frame.ID,
		CreatedAtNs:      frame.DeadlineNs,
		LeftSampleID:     left.ID,
		RightSampleID:    right.ID,
		LeftPoseVersion:  left.Version,
		RightPoseVersion: right.Version,
		LeftYawDeg:       left.YawDeg,
		LeftPitchDeg:     left.PitchDeg,
		LeftRollDeg:      left.RollDeg,
		RightYawDeg:      right.YawDeg,
		RightPitchDeg:    right.PitchDeg,
		RightRollDeg:     right.RollDeg,
		LeftEye:          eyeMatrix(left, -1),
		RightEye:         eyeMatrix(right, 1),
	}
	record.Digest = matrixDigest(record)
	return record
}

func eyeMatrix(sample *acceptedSample, side float64) Matrix4 {
	yaw := sample.YawDeg * math.Pi / 180
	pitch := sample.PitchDeg * math.Pi / 180
	roll := sample.RollDeg * math.Pi / 180

	cy, sy := math.Cos(yaw), math.Sin(yaw)
	cp, sp := math.Cos(pitch), math.Sin(pitch)
	cr, sr := math.Cos(roll), math.Sin(roll)

	// R = Rz(roll) * Ry(yaw) * Rx(pitch), with a fixed IPD offset.
	m := Matrix4{
		cy*cr + sy*sp*sr, -cy*sr + sy*sp*cr, sy * cp, 0,
		cp * sr, cp * cr, -sp, 0,
		-sy*cr + cy*sp*sr, sy*sr + cy*sp*cr, cy * cp, 0,
		0.032 * side, 0, 0.064, 1,
	}
	return m
}

func matrixDigest(record MatrixRecord) string {
	var values []string
	for _, value := range record.LeftEye {
		values = append(values, fmt.Sprintf("%.9f", value))
	}
	for _, value := range record.RightEye {
		values = append(values, fmt.Sprintf("%.9f", value))
	}
	sum := sha256.Sum256([]byte(strings.Join(values, ",")))
	return fmt.Sprintf("%x", sum[:])
}

func matrixSummary(record *MatrixRecord) string {
	if record == nil {
		return "none"
	}
	return fmt.Sprintf("M%d(l%d/r%d)", record.Version, record.LeftPoseVersion, record.RightPoseVersion)
}
