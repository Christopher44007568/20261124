package replay

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func WriteResult(directory string, result *Replay) error {
	if result == nil {
		return fmt.Errorf("nil replay result")
	}
	if err := os.MkdirAll(filepath.Join(directory, "frames"), 0o755); err != nil {
		return err
	}
	for _, frame := range result.Frames {
		image, ok := result.Images[frame.ID]
		if !ok {
			return fmt.Errorf("frame %s has no rendered image", frame.ID)
		}
		path := filepath.Join(directory, filepath.FromSlash(frame.ImagePath))
		if err := os.WriteFile(path, image, 0o644); err != nil {
			return err
		}
	}
	if err := writeJSON(filepath.Join(directory, "replay.json"), result); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(directory, "summary.json"), result.Summary); err != nil {
		return err
	}
	if err := writeTimeline(filepath.Join(directory, "timeline-submit.jsonl"), result.Timelines.Submit); err != nil {
		return err
	}
	if err := writeTimeline(filepath.Join(directory, "timeline-deadline.jsonl"), result.Timelines.Deadline); err != nil {
		return err
	}
	if err := writeTimeline(filepath.Join(directory, "timeline-composite.jsonl"), result.Timelines.Composite); err != nil {
		return err
	}
	if err := writeTimeline(filepath.Join(directory, "timeline-scanout.jsonl"), result.Timelines.Scanout); err != nil {
		return err
	}
	return writeAudit(filepath.Join(directory, "audit.csv"), result)
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func writeTimeline[T any](path string, events []T) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	for _, event := range events {
		if err := encoder.Encode(event); err != nil {
			return err
		}
	}
	return nil
}

func writeAudit(path string, result *Replay) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	header := []string{
		"frameId", "submitNs", "deadlineNs", "compositeNs", "scanoutNs",
		"matrixVersion", "leftPoseVersion", "rightPoseVersion",
		"matrixReplaced", "resolution", "trackingState",
		"imagePath", "imageSha256", "oldResourceReadIds", "oldProducerMatrices",
	}
	if err := writer.Write(header); err != nil {
		return err
	}
	for _, frame := range result.Frames {
		var producers []string
		for _, readID := range frame.OldResourceReadIDs {
			for _, read := range result.ResourceReads {
				if read.ID == readID {
					producers = append(producers, strconv.Itoa(read.ProducerMatrixVersion))
				}
			}
		}
		row := []string{
			frame.ID,
			strconv.FormatInt(frame.SubmitNs, 10),
			strconv.FormatInt(frame.DeadlineNs, 10),
			strconv.FormatInt(frame.CompositeNs, 10),
			strconv.FormatInt(frame.ScanoutNs, 10),
			strconv.Itoa(frame.UsedMatrixVersion),
			strconv.Itoa(frame.UsedLeftPoseVersion),
			strconv.Itoa(frame.UsedRightPoseVersion),
			strconv.FormatBool(frame.MatrixReplaced),
			frame.Resolution,
			frame.TrackingState,
			frame.ImagePath,
			frame.ImageSHA256,
			strings.Join(frame.OldResourceReadIDs, ";"),
			strings.Join(producers, ";"),
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}
