//go:build !windows

package main

import "fmt"

type NativeCaptureRequest struct {
	ProjectID         string `json:"project_id"`
	FPS               int    `json:"fps"`
	AudioDevice       string `json:"audio_device"`
	CaptureCursor     bool   `json:"capture_cursor"`
	CaptureKeystrokes bool   `json:"capture_keystrokes"`
	Reconnect         bool   `json:"reconnect"`
}
type NativeCaptureCapabilitiesResponse struct {
	Supported          bool     `json:"supported"`
	FFmpegAvailable    bool     `json:"ffmpeg_available"`
	AudioDevices       []string `json:"audio_devices"`
	VideoDevices       []string `json:"video_devices"`
	SystemAudioDevices []string `json:"system_audio_devices"`
	Reason             string   `json:"reason,omitempty"`
}

func (a *App) NativeCaptureCapabilities() NativeCaptureCapabilitiesResponse {
	return NativeCaptureCapabilitiesResponse{Supported: false, AudioDevices: []string{}, VideoDevices: []string{}, SystemAudioDevices: []string{}, Reason: "Native desktop capture is currently implemented for Windows."}
}
func (a *App) StartNativeCapture(NativeCaptureRequest) (map[string]any, error) {
	return nil, fmt.Errorf("native capture is unavailable on this platform")
}
func (a *App) StopNativeCapture(string) (map[string]any, error) {
	return nil, fmt.Errorf("native capture is unavailable on this platform")
}
func (a *App) ImportNativeCapture(string, string) (map[string]any, error) {
	return nil, fmt.Errorf("native capture is unavailable on this platform")
}
