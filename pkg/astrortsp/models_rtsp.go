package astrortsp

import (
	"context"
	"time"
)

// =========== RTSP MODELS ========
type RtspConfig struct {
	ID        string
	RTSPUrl   string
	OutputDir string
	Timeout   time.Duration
	Context   context.Context
}
