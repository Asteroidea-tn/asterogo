package astrortsp

import (
	"fmt"
	"path/filepath"
	"time"
)

// captureSplit captures an image and splits it vertically/horizontally
// splitPos = 0 → center split
func (s *SnapshotService) CaptureSplit(vertical bool, splitPos int) (string, string, error) {
	imgPath := func(suffix string) string {
		ts := time.Now().Format("2006-01-02_15-04-05")
		return filepath.Join(s.RtspCamera.OutputDir, fmt.Sprintf("%s_%s_%s.jpg", s.RtspCamera.ID, ts, suffix))
	}

	var filter1, filter2 string
	if vertical {
		if splitPos == 0 {
			filter1 = "crop=iw/2:ih:0:0"
			filter2 = "crop=iw/2:ih:iw/2:0"
		} else {
			filter1 = fmt.Sprintf("crop=%d:ih:0:0", splitPos)
			filter2 = fmt.Sprintf("crop=iw-%d:ih:%d:0", splitPos, splitPos)
		}
	} else { // horizontal
		if splitPos == 0 {
			filter1 = "crop=iw:ih/2:0:0"
			filter2 = "crop=iw:ih/2:0:ih/2"
		} else {
			filter1 = fmt.Sprintf("crop=iw:%d:0:0", splitPos)
			filter2 = fmt.Sprintf("crop=iw:ih-%d:0:%d", splitPos, splitPos)
		}
	}

	out1 := imgPath("1")
	out2 := imgPath("2")

	if err := s.captureAndSaveWithFilter(s.RtspCamera.Context, filter1, out1); err != nil {
		return "", "", fmt.Errorf("error saving first split: %w", err)
	}
	if err := s.captureAndSaveWithFilter(s.RtspCamera.Context, filter2, out2); err != nil {
		return "", "", fmt.Errorf("error saving second split: %w", err)
	}

	return out1, out2, nil
}

// CaptureCrop captures an image and crops it to a specific rectangle
func (s *SnapshotService) CaptureCrop(x, y, w, h int) (string, error) {
	filter := fmt.Sprintf("crop=%d:%d:%d:%d", w, h, x, y)
	outFile := filepath.Join(s.RtspCamera.OutputDir, fmt.Sprintf("%s_crop_%s.jpg", s.RtspCamera.ID, time.Now().Format("2006-01-02_15-04-05")))
	if err := s.captureAndSaveWithFilter(s.RtspCamera.Context, filter, outFile); err != nil {
		return "", err
	}
	return outFile, nil
}
