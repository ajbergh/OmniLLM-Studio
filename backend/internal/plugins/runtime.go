package plugins

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

const (
	pluginInitializeTimeout = 10 * time.Second
	pluginShutdownTimeout   = 3 * time.Second
	pluginMaxMessageBytes   = 1 << 20
)

// JSONRPCRequest is a JSON-RPC 2.0 request sent to a plugin subprocess.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCResponse is a JSON-RPC 2.0 response from a plugin subprocess.
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC error object.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type pluginResponse struct {
	result json.RawMessage
	err    error
}

// PluginProcess manages the lifecycle of a plugin subprocess.
//
// State and blocking I/O intentionally use separate locks. No process read,
// write, wait, or shutdown operation may execute while mu is held.
type PluginProcess struct {
	manifest   *models.PluginManifest
	entrypoint string

	mu      sync.Mutex
	writeMu sync.Mutex
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	pending map[int]chan pluginResponse
	nextID  int
	running bool
	done    chan struct{}
	doneErr error
}

// NewPluginProcess creates a new plugin process (not yet started).
// Returns nil if the entrypoint escapes the plugin directory.
func NewPluginProcess(manifest *models.PluginManifest, pluginDir string) *PluginProcess {
	if manifest == nil || strings.TrimSpace(manifest.Entrypoint) == "" {
		log.Printf("WARN: plugin manifest or entrypoint is empty")
		return nil
	}

	entrypoint := manifest.Entrypoint
	if entrypoint[0] == '.' {
		entrypoint = filepath.Join(pluginDir, entrypoint)
	}

	absEntry, err := filepath.Abs(entrypoint)
	if err != nil {
		log.Printf("WARN: plugin entrypoint %q: cannot resolve absolute path", entrypoint)
		return nil
	}
	absPlugin, err := filepath.Abs(pluginDir)
	if err != nil {
		log.Printf("WARN: plugin directory %q: cannot resolve absolute path", pluginDir)
		return nil
	}
	entryEval, err := resolvePluginPath(absPlugin, absEntry)
	if err != nil {
		log.Printf("WARN: plugin entrypoint %q: cannot resolve safely: %v", entrypoint, err)
		return nil
	}
	pluginEval, err := resolvePluginPath(absPlugin, absPlugin)
	if err != nil {
		log.Printf("WARN: plugin directory %q: cannot resolve safely: %v", pluginDir, err)
		return nil
	}
	rel, err := filepath.Rel(pluginEval, entryEval)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		log.Printf("WARN: plugin entrypoint %q escapes plugin dir, refusing to start", entrypoint)
		return nil
	}

	return &PluginProcess{
		manifest:   manifest,
		entrypoint: entryEval,
		nextID:     1,
		pending:    make(map[int]chan pluginResponse),
	}
}

// resolvePluginPath canonicalizes a plugin path. On Windows, EvalSymlinks can
// return access denied for a usable path in a restricted temporary directory.
// In that narrow case we retain containment by accepting only a lexical child
// of pluginDir with no symlink component; all other resolution errors reject
// the plugin.
func resolvePluginPath(pluginDir, candidate string) (string, error) {
	resolved, err := filepath.EvalSymlinks(candidate)
	if err == nil {
		return resolved, nil
	}
	if runtime.GOOS != "windows" || !errors.Is(err, fs.ErrPermission) {
		return "", err
	}

	rel, relErr := filepath.Rel(pluginDir, candidate)
	if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("candidate escapes plugin directory")
	}
	if err := rejectSymlinkPath(pluginDir, rel); err != nil {
		return "", err
	}
	return filepath.Clean(candidate), nil
}

func rejectSymlinkPath(root, relativePath string) error {
	current := root
	parts := []string{"."}
	if relativePath != "." {
		parts = strings.Split(relativePath, string(filepath.Separator))
	}
	for _, part := range parts {
		if part != "." {
			current = filepath.Join(current, part)
		}
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink component %q is not permitted", current)
		}
	}
	return nil
}

// Start launches the plugin subprocess and performs bounded initialization.
func (p *PluginProcess) Start() error {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return fmt.Errorf("plugin %q already running", p.manifest.Name)
	}

	cmd := exec.Command(p.entrypoint)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		p.mu.Unlock()
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		p.mu.Unlock()
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		p.mu.Unlock()
		return fmt.Errorf("stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		p.mu.Unlock()
		return fmt.Errorf("start plugin %q: %w", p.manifest.Name, err)
	}

	p.cmd = cmd
	p.stdin = stdin
	p.pending = make(map[int]chan pluginResponse)
	p.done = make(chan struct{})
	p.doneErr = nil
	p.running = true
	p.mu.Unlock()

	go p.readLoop(stdout)
	go p.stderrLoop(stderr)
	go p.waitLoop(cmd)

	ctx, cancel := context.WithTimeout(context.Background(), pluginInitializeTimeout)
	defer cancel()
	if _, err := p.Call(ctx, "initialize", json.RawMessage(`{}`)); err != nil {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), pluginShutdownTimeout)
		defer stopCancel()
		_ = p.StopContext(stopCtx)
		return fmt.Errorf("initialize plugin %q: %w", p.manifest.Name, err)
	}
	return nil
}

// Stop shuts down the plugin subprocess with a bounded timeout.
func (p *PluginProcess) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), pluginShutdownTimeout)
	defer cancel()
	return p.StopContext(ctx)
}

// StopContext shuts down the plugin subprocess and kills it if it does not exit
// before ctx is done.
func (p *PluginProcess) StopContext(ctx context.Context) error {
	p.mu.Lock()
	if !p.running {
		err := p.doneErr
		p.mu.Unlock()
		return normalizePluginStopError(err)
	}
	cmd := p.cmd
	stdin := p.stdin
	done := p.done
	p.mu.Unlock()

	shutdownCtx, cancel := context.WithTimeout(ctx, time.Second)
	_, _ = p.Call(shutdownCtx, "shutdown", json.RawMessage(`{}`))
	cancel()
	if stdin != nil {
		_ = stdin.Close()
	}

	select {
	case <-done:
		p.mu.Lock()
		err := p.doneErr
		p.mu.Unlock()
		return normalizePluginStopError(err)
	case <-ctx.Done():
		if cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		select {
		case <-done:
		case <-time.After(time.Second):
		}
		return ctx.Err()
	}
}

// Call sends a JSON-RPC request and waits for the matching response or context
// cancellation. Concurrent calls are supported.
func (p *PluginProcess) Call(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	p.mu.Lock()
	if !p.running || p.stdin == nil || p.done == nil {
		p.mu.Unlock()
		return nil, fmt.Errorf("plugin %q not running", p.manifest.Name)
	}
	id := p.nextID
	p.nextID++
	responseCh := make(chan pluginResponse, 1)
	p.pending[id] = responseCh
	stdin := p.stdin
	done := p.done
	p.mu.Unlock()

	req := JSONRPCRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}
	data, err := json.Marshal(req)
	if err != nil {
		p.removePending(id)
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	data = append(data, '\n')

	p.writeMu.Lock()
	_, err = stdin.Write(data)
	p.writeMu.Unlock()
	if err != nil {
		p.removePending(id)
		return nil, fmt.Errorf("write to plugin: %w", err)
	}

	select {
	case response := <-responseCh:
		return response.result, response.err
	case <-ctx.Done():
		p.removePending(id)
		return nil, ctx.Err()
	case <-done:
		p.removePending(id)
		p.mu.Lock()
		doneErr := p.doneErr
		p.mu.Unlock()
		if doneErr != nil {
			return nil, doneErr
		}
		return nil, io.EOF
	}
}

func (p *PluginProcess) readLoop(stdout io.Reader) {
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), pluginMaxMessageBytes)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var resp JSONRPCResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			log.Printf("[plugin:%s] invalid JSON-RPC response: %v", p.manifest.Name, err)
			continue
		}
		p.completePending(resp)
	}
	if err := scanner.Err(); err != nil {
		p.failPending(fmt.Errorf("read from plugin: %w", err))
	}
}

func (p *PluginProcess) stderrLoop(stderr io.Reader) {
	scanner := bufio.NewScanner(stderr)
	scanner.Buffer(make([]byte, 8*1024), 256*1024)
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" {
			log.Printf("[plugin:%s] %s", p.manifest.Name, line)
		}
	}
}

func (p *PluginProcess) waitLoop(cmd *exec.Cmd) {
	err := cmd.Wait()
	p.mu.Lock()
	if p.cmd != cmd {
		p.mu.Unlock()
		return
	}
	p.running = false
	p.stdin = nil
	p.doneErr = err
	done := p.done
	pending := p.pending
	p.pending = make(map[int]chan pluginResponse)
	p.mu.Unlock()

	for _, ch := range pending {
		ch <- pluginResponse{err: normalizePluginExitError(err)}
	}
	close(done)
}

func (p *PluginProcess) completePending(resp JSONRPCResponse) {
	p.mu.Lock()
	ch, ok := p.pending[resp.ID]
	if ok {
		delete(p.pending, resp.ID)
	}
	p.mu.Unlock()
	if !ok {
		return
	}
	if resp.Error != nil {
		ch <- pluginResponse{err: fmt.Errorf("plugin error %d: %s", resp.Error.Code, resp.Error.Message)}
		return
	}
	ch <- pluginResponse{result: resp.Result}
}

func (p *PluginProcess) removePending(id int) {
	p.mu.Lock()
	delete(p.pending, id)
	p.mu.Unlock()
}

func (p *PluginProcess) failPending(err error) {
	p.mu.Lock()
	pending := p.pending
	p.pending = make(map[int]chan pluginResponse)
	p.mu.Unlock()
	for _, ch := range pending {
		ch <- pluginResponse{err: err}
	}
}

func normalizePluginExitError(err error) error {
	if err == nil {
		return io.EOF
	}
	return fmt.Errorf("plugin %q exited: %w", "process", err)
}

func normalizePluginStopError(err error) error {
	if err == nil || err == io.EOF {
		return nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 0 {
		return nil
	}
	return err
}

// Manifest returns the plugin's manifest.
func (p *PluginProcess) Manifest() *models.PluginManifest {
	return p.manifest
}

// IsRunning returns whether the plugin process is active.
func (p *PluginProcess) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}
