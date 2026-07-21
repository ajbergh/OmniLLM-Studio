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
	"sort"
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
	// Reconnect is reserved for a future segmented DirectShow reconnect flow.
	// It is rejected today rather than silently claiming continuity.
	Reconnect bool `json:"reconnect"`
}

type NativeCaptureCapabilitiesResponse struct {
	Supported          bool     `json:"supported"`
	FFmpegAvailable    bool     `json:"ffmpeg_available"`
	AudioDevices       []string `json:"audio_devices"`
	VideoDevices       []string `json:"video_devices"`
	SystemAudioDevices []string `json:"system_audio_devices"`
	ReconnectSupported bool     `json:"reconnect_supported"`
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
		return NativeCaptureCapabilitiesResponse{
			Supported: false, FFmpegAvailable: false,
			AudioDevices: []string{}, VideoDevices: []string{}, SystemAudioDevices: []string{},
			ReconnectSupported: false,
			Reason:             "FFmpeg was not found on PATH.",
		}
	}
	audio, video := listDShowDevices()
	loopback := make([]string, 0)
	for _, name := range audio {
		lower := strings.ToLower(name)
		if strings.Contains(lower, "stereo mix") || strings.Contains(lower, "loopback") || strings.Contains(lower, "what u hear") {
			loopback = append(loopback, name)
		}
	}
	return NativeCaptureCapabilitiesResponse{
		Supported: true, FFmpegAvailable: true,
		AudioDevices: audio, VideoDevices: video, SystemAudioDevices: loopback,
		ReconnectSupported: false,
	}
}

func listDShowDevices() ([]string, []string) {
	cmd := exec.Command("ffmpeg", "-hide_banner", "-list_devices", "true", "-f", "dshow", "-i", "dummy")
	output, _ := cmd.CombinedOutput()
	return parseDShowDevices(output)
}

func parseDShowDevices(output []byte) ([]string, []string) {
	scanner := bufio.NewScanner(bytes.NewReader(output))
	pattern := regexp.MustCompile(`"([^"]+)"`)
	audio, video := []string{}, []string{}
	kind := ""
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.Contains(line, "DirectShow video devices"):
			kind = "video"
		case strings.Contains(line, "DirectShow audio devices"):
			kind = "audio"
		}
		matches := pattern.FindStringSubmatch(line)
		if len(matches) != 2 || strings.Contains(strings.ToLower(line), "alternative name") {
			continue
		}
		switch kind {
		case "audio":
			audio = append(audio, matches[1])
		case "video":
			video = append(video, matches[1])
		}
	}
	return uniqueStrings(audio), uniqueStrings(video)
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (a *App) StartNativeCapture(request NativeCaptureRequest) (map[string]any, error) {
	request.ProjectID = strings.TrimSpace(request.ProjectID)
	request.AudioDevice = strings.TrimSpace(request.AudioDevice)
	if request.ProjectID == "" {
		return nil, fmt.Errorf("project_id is required")
	}
	if request.Reconnect {
		return nil, fmt.Errorf("automatic DirectShow reconnect is not supported; restart capture after reconnecting the device")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, fmt.Errorf("FFmpeg was not found on PATH: %w", err)
	}
	if request.FPS <= 0 {
		request.FPS = 30
	}
	if request.FPS > 60 {
		request.FPS = 60
	}
	if request.AudioDevice != "" {
		audio, _ := listDShowDevices()
		if !containsString(audio, request.AudioDevice) {
			return nil, fmt.Errorf("selected audio device is no longer available; refresh devices and try again")
		}
	}

	id := uuid.NewString()
	dir := filepath.Join(desktopDataDir(), "captures")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create capture directory: %w", err)
	}
	output := filepath.Join(dir, id+".mp4")
	telemetry := filepath.Join(dir, id+".events.jsonl")
	args := []string{
		"-y", "-hide_banner", "-loglevel", "warning",
		"-f", "gdigrab", "-framerate", fmt.Sprint(request.FPS), "-draw_mouse", "1", "-i", "desktop",
	}
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
		return nil, fmt.Errorf("open FFmpeg control pipe: %w", err)
	}
	cmd.Stdout = io.Discard
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("start FFmpeg capture: %w", err)
	}
	session := &nativeCaptureSession{
		id: id, request: request, outputPath: output, telemetryPath: telemetry,
		cmd: cmd, stdin: stdin, done: make(chan error, 1),
		stopTelemetry: make(chan struct{}), started: time.Now(),
	}
	nativeCaptureState.Lock()
	nativeCaptureState.sessions[id] = session
	nativeCaptureState.Unlock()
	go func() {
		err := cmd.Wait()
		if err != nil && stderr.Len() > 0 {
			err = fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
		}
		session.done <- err
	}()
	if request.CaptureCursor || request.CaptureKeystrokes {
		go recordNativeTelemetry(session)
	}
	return map[string]any{"session_id": id, "status": "recording"}, nil
}

func stopNativeCaptureSession(session *nativeCaptureSession, timeout time.Duration) error {
	if session == nil {
		return fmt.Errorf("capture session not found")
	}
	session.stopOnce.Do(func() { close(session.stopTelemetry) })
	_, _ = io.WriteString(session.stdin, "q\n")
	_ = session.stdin.Close()
	select {
	case err := <-session.done:
		if err != nil {
			return fmt.Errorf("FFmpeg capture failed: %w", err)
		}
		return nil
	case <-time.After(timeout):
		if session.cmd.Process != nil {
			_ = session.cmd.Process.Kill()
		}
		return fmt.Errorf("capture did not stop cleanly within %s", timeout)
	}
}

func (a *App) StopNativeCapture(id string) (map[string]any, error) {
	nativeCaptureState.Lock()
	session := nativeCaptureState.sessions[id]
	nativeCaptureState.Unlock()
	if err := stopNativeCaptureSession(session, 15*time.Second); err != nil {
		return nil, err
	}
	info, err := os.Stat(session.outputPath)
	if err != nil {
		return nil, fmt.Errorf("inspect native capture output: %w", err)
	}
	return map[string]any{
		"session_id": id, "status": "completed", "size_bytes": info.Size(),
		"telemetry_available": fileExists(session.telemetryPath),
	}, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (a *App) ImportNativeCapture(id, projectID string) (map[string]any, error) {
	projectID = strings.TrimSpace(projectID)
	nativeCaptureState.Lock()
	session := nativeCaptureState.sessions[id]
	nativeCaptureState.Unlock()
	if session == nil {
		return nil, fmt.Errorf("capture session not found")
	}
	if projectID == "" || projectID != session.request.ProjectID {
		return nil, fmt.Errorf("native capture can only be imported into its originating project")
	}
	file, err := os.Open(session.outputPath)
	if err != nil {
		return nil, fmt.Errorf("open native capture: %w", err)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filepath.Base(session.outputPath))
	if err == nil {
		_, err = io.Copy(part, file)
	}
	closeErr := file.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	url := strings.TrimRight(a.apiBase, "/") + "/video/projects/" + projectID + "/assets/upload"
	req, err := http.NewRequest(http.MethodPost, url, &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	client := &http.Client{Timeout: 15 * time.Minute}
	response, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upload native capture: %w", err)
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("native capture import returned %d: %s", response.StatusCode, strings.TrimSpace(string(data)))
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode native capture import: %w", err)
	}

	nativeCaptureState.Lock()
	delete(nativeCaptureState.sessions, id)
	nativeCaptureState.Unlock()
	_ = os.Remove(session.outputPath)
	_ = os.Remove(session.telemetryPath)
	return result, nil
}

// shutdownNativeCaptures stops all active FFmpeg capture processes before the
// desktop runtime exits. Captured files remain in the private capture directory
// so an interrupted session can be recovered manually rather than discarded.
func shutdownNativeCaptures() {
	nativeCaptureState.Lock()
	sessions := make([]*nativeCaptureSession, 0, len(nativeCaptureState.sessions))
	for _, session := range nativeCaptureState.sessions {
		sessions = append(sessions, session)
	}
	nativeCaptureState.sessions = map[string]*nativeCaptureSession{}
	nativeCaptureState.Unlock()
	for _, session := range sessions {
		_ = stopNativeCaptureSession(session, 5*time.Second)
	}
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
				// Store virtual key codes, not decoded text. The explicit UI opt-in
				// remains required because key timing can still be sensitive.
				virtualKeys := make([]int, 0)
				for code := uintptr(0x08); code <= 0xFE; code++ {
					state, _, _ := procGetAsyncKeyState.Call(code)
					if state&1 != 0 {
						virtualKeys = append(virtualKeys, int(code))
					}
				}
				if len(virtualKeys) > 0 {
					event["virtual_keys"] = virtualKeys
				}
			}
			_ = encoder.Encode(event)
		case <-session.stopTelemetry:
			return
		}
	}
}
