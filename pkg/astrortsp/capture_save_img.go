package astrortsp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type SnapshotService struct {
	RtspCamera RtspConfig
}

func NewSnapshotService(cfg RtspConfig) *SnapshotService {
	_ = os.MkdirAll(cfg.OutputDir, 0755)
	return &SnapshotService{RtspCamera: cfg}
}

// CaptureImg captures a single image from the RTSP stream and saves it to a file
func (s *SnapshotService) CaptureImg() (string, error) {
	ts := time.Now().Format("2006-01-02_15-04-05")
	outFile := filepath.Join(s.RtspCamera.OutputDir, fmt.Sprintf("%s_%s.jpg", s.RtspCamera.ID, ts))

	// Only pass -vf if filter is needed; empty string = no filter
	err := s.captureAndSaveWithFilter(s.RtspCamera.Context, "", outFile)
	if err != nil {
		return "", err
	}

	return outFile, nil
}

// captureAndSaveWithFilter captures an image applying the given video filter (vf) and saves it to filename
func (s *SnapshotService) captureAndSaveWithFilter(ctx context.Context, vf string, filename string) error {
	ctx, cancel := context.WithTimeout(s.RtspCamera.Context, s.RtspCamera.Timeout)
	defer cancel()

	args := []string{
		"-rtsp_transport", "tcp",
		"-i", s.RtspCamera.RTSPUrl,
		"-frames:v", "1",
		"-q:v", "2",
		filename,
	}

	if vf != "" {
		// insert filter before output file
		args = []string{
			"-rtsp_transport", "tcp",
			"-i", s.RtspCamera.RTSPUrl,
			"-frames:v", "1",
			"-vf", vf,
			"-q:v", "2",
			filename,
		}
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg error: %w | %s", err, string(out))
	}

	return nil
}

// saveImg writes the byte slice to the output file
func (s *SnapshotService) SaveImg(data []byte, filename string) error {
	if err := os.MkdirAll(s.RtspCamera.OutputDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}
