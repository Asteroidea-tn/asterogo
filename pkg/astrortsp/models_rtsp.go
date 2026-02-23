// ================ Version : V1.1.0 ===========
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
