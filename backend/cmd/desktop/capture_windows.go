//go:build windows

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/google/uuid"
	"golang.org/x/sys/windows"
)

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
type nativeCaptureSession struct {
	id                        string
	request                   NativeCaptureRequest
	outputPath, telemetryPath string
	cmd                       *exec.Cmd
	stdin                     io.WriteCloser
	done                      chan error
	stopTelemetry             chan struct{}
	stopOnce                  sync.Once
	started                   time.Time
}

var nativeCaptureState = struct {
	sync.Mutex
	sessions map[string]*nativeCaptureSession
}{sessions: map[string]*nativeCaptureSession{}}

func (a *App) NativeCaptureCapabilities() NativeCaptureCapabilitiesResponse {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return NativeCaptureCapabilitiesResponse{Supported: false, FFmpegAvailable: false, AudioDevices: []string{}, VideoDevices: []string{}, SystemAudioDevices: []string{}, Reason: "FFmpeg was not found on PATH."}
	}
	audio, video := listDShowDevices()
	loopback := []string{}
	for _, name := range audio {
		lower := strings.ToLower(name)
		if strings.Contains(lower, "stereo mix") || strings.Contains(lower, "loopback") || strings.Contains(lower, "what u hear") {
			loopback = append(loopback, name)
		}
	}
	return NativeCaptureCapabilitiesResponse{Supported: true, FFmpegAvailable: true, AudioDevices: audio, VideoDevices: video, SystemAudioDevices: loopback}
}
func listDShowDevices() ([]string, []string) {
	cmd := exec.Command("ffmpeg", "-hide_banner", "-list_devices", "true", "-f", "dshow", "-i", "dummy")
	output, _ := cmd.CombinedOutput()
	scanner := bufio.NewScanner(bytes.NewReader(output))
	pattern := regexp.MustCompile(`"([^"]+)"`)
	audio, video := []string{}, []string{}
	kind := ""
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "DirectShow video devices") {
			kind = "video"
		}
		if strings.Contains(line, "DirectShow audio devices") {
			kind = "audio"
		}
		matches := pattern.FindStringSubmatch(line)
		if len(matches) == 2 {
			if kind == "audio" {
				audio = append(audio, matches[1])
			} else if kind == "video" {
				video = append(video, matches[1])
			}
		}
	}
	return uniqueStrings(audio), uniqueStrings(video)
}
func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

func (a *App) StartNativeCapture(request NativeCaptureRequest) (map[string]any, error) {
	if strings.TrimSpace(request.ProjectID) == "" {
		return nil, fmt.Errorf("project_id is required")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, err
	}
	if request.FPS <= 0 {
		request.FPS = 30
	}
	if request.FPS > 60 {
		request.FPS = 60
	}
	id := uuid.NewString()
	dir := filepath.Join(desktopDataDir(), "captures")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	output := filepath.Join(dir, id+".mp4")
	telemetry := filepath.Join(dir, id+".events.jsonl")
	args := []string{"-y", "-hide_banner", "-f", "gdigrab", "-framerate", fmt.Sprint(request.FPS), "-draw_mouse", "1", "-i", "desktop"}
	if request.AudioDevice != "" {
		args = append(args, "-f", "dshow", "-i", "audio="+request.AudioDevice)
	}
	args = append(args, "-c:v", "libx264", "-preset", "veryfast", "-pix_fmt", "yuv420p")
	if request.AudioDevice != "" {
		args = append(args, "-c:a", "aac", "-b:a", "192k")
	} else {
		args = append(args, "-an")
	}
	args = append(args, "-movflags", "+faststart", output)
	cmd := exec.Command("ffmpeg", args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stdout = io.Discard
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start FFmpeg capture: %w", err)
	}
	session := &nativeCaptureSession{id: id, request: request, outputPath: output, telemetryPath: telemetry, cmd: cmd, stdin: stdin, done: make(chan error, 1), stopTelemetry: make(chan struct{}), started: time.Now()}
	nativeCaptureState.Lock()
	nativeCaptureState.sessions[id] = session
	nativeCaptureState.Unlock()
	go func() { session.done <- cmd.Wait() }()
	if request.CaptureCursor || request.CaptureKeystrokes {
		go recordNativeTelemetry(session)
	}
	return map[string]any{"session_id": id, "status": "recording", "output_path": output}, nil
}

func (a *App) StopNativeCapture(id string) (map[string]any, error) {
	nativeCaptureState.Lock()
	session := nativeCaptureState.sessions[id]
	nativeCaptureState.Unlock()
	if session == nil {
		return nil, fmt.Errorf("capture session not found")
	}
	_, _ = io.WriteString(session.stdin, "q\n")
	_ = session.stdin.Close()
	select {
	case err := <-session.done:
		session.stopOnce.Do(func() { close(session.stopTelemetry) })
		if err != nil {
			return nil, fmt.Errorf("FFmpeg capture failed: %w", err)
		}
	case <-time.After(15 * time.Second):
		session.stopOnce.Do(func() { close(session.stopTelemetry) })
		_ = session.cmd.Process.Kill()
		return nil, fmt.Errorf("capture did not stop cleanly")
	}
	info, err := os.Stat(session.outputPath)
	if err != nil {
		return nil, err
	}
	return map[string]any{"session_id": id, "status": "completed", "output_path": session.outputPath, "telemetry_path": session.telemetryPath, "size_bytes": info.Size()}, nil
}

func (a *App) ImportNativeCapture(id, projectID string) (map[string]any, error) {
	nativeCaptureState.Lock()
	session := nativeCaptureState.sessions[id]
	nativeCaptureState.Unlock()
	if session == nil {
		return nil, fmt.Errorf("capture session not found")
	}
	file, err := os.Open(session.outputPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filepath.Base(session.outputPath))
	if err != nil {
		return nil, err
	}
	if _, err = io.Copy(part, file); err != nil {
		return nil, err
	}
	_ = writer.Close()
	url := strings.TrimRight(a.apiBase, "/") + "/video/projects/" + projectID + "/assets/upload"
	req, err := http.NewRequest(http.MethodPost, url, &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("native capture import returned %d: %s", response.StatusCode, string(data))
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	nativeCaptureState.Lock()
	delete(nativeCaptureState.sessions, id)
	nativeCaptureState.Unlock()
	return result, nil
}

var user32 = windows.NewLazySystemDLL("user32.dll")
var procGetCursorPos = user32.NewProc("GetCursorPos")
var procGetAsyncKeyState = user32.NewProc("GetAsyncKeyState")

type nativePoint struct{ X, Y int32 }

func recordNativeTelemetry(session *nativeCaptureSession) {
	file, err := os.OpenFile(session.telemetryPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	ticker := time.NewTicker(33 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			event := map[string]any{"time_ms": time.Since(session.started).Milliseconds()}
			if session.request.CaptureCursor {
				var point nativePoint
				procGetCursorPos.Call(uintptr(unsafe.Pointer(&point)))
				event["cursor_x"] = point.X
				event["cursor_y"] = point.Y
				left, _, _ := procGetAsyncKeyState.Call(0x01)
				event["click"] = (left & 0x8000) != 0
			}
			if session.request.CaptureKeystrokes {
				keys := []string{}
				for code := uintptr(0x30); code <= 0x5A; code++ {
					state, _, _ := procGetAsyncKeyState.Call(code)
					if state&1 != 0 {
						keys = append(keys, fmt.Sprintf("%c", rune(code)))
					}
				}
				if len(keys) > 0 {
					event["keys"] = keys
				}
			}
			_ = encoder.Encode(event)
		case <-session.stopTelemetry:
			return
		}
	}
}
