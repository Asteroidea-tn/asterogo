package astrortsp

import (
	"log"
	"time"
)

// example
func GetImageSnapshot(camID string) string {

	// get image by cam id from config to pass to RTSP service

	camera := RtspConfig{
		ID:        camID,
		RTSPUrl:   "rtsp://admin:admin@20.0.0.65:554/Streaming/channels/101",
		OutputDir: "./data/28-01",
		Timeout:   10 * time.Second,
	}

	service := NewSnapshotService(camera)

	imgPath, err := service.CaptureImg()
	if err != nil {
		log.Fatalf("CaptureImg error: %v", err)
	}
	log.Printf("1️ Single image captured: %s", imgPath)

	return imgPath

}
