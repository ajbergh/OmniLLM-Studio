package plugins

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ajbergh/omnillm-studio/internal/models"
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

// PluginProcess manages the lifecycle of a plugin subprocess.
type PluginProcess struct {
	manifest *models.PluginManifest
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	scanner  *bufio.Scanner
	mu       sync.Mutex
	nextID   int
	running  bool
}

// NewPluginProcess creates a new plugin process (not yet started).
// Returns nil if the entrypoint escapes the plugin directory.
func NewPluginProcess(manifest *models.PluginManifest, pluginDir string) *PluginProcess {
	entrypoint := manifest.Entrypoint
	// Resolve relative entrypoints against plugin directory
	if entrypoint != "" && entrypoint[0] == '.' {
		entrypoint = filepath.Join(pluginDir, entrypoint)
	}

	// Validate entrypoint is under plugin directory
	absEntry, err := filepath.Abs(entrypoint)
	if err != nil {
		log.Printf("WARN: plugin entrypoint %q: cannot resolve absolute path", entrypoint)
		return nil
	}
	absPlugin, _ := filepath.Abs(pluginDir)
	rel, err := filepath.Rel(absPlugin, absEntry)
	if err != nil || strings.HasPrefix(rel, "..") {
		log.Printf("WARN: plugin entrypoint %q escapes plugin dir, refusing to start", entrypoint)
		return nil
	}

	return &PluginProcess{
		manifest: manifest,
		cmd:      exec.Command(absEntry),
		nextID:   1,
	}
}

// Start launches the plugin subprocess.
func (p *PluginProcess) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return fmt.Errorf("plugin %q already running", p.manifest.Name)
	}

	stdin, err := p.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := p.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	p.stdin = stdin
	p.scanner = bufio.NewScanner(stdout)
	p.scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer

	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf("start plugin %q: %w", p.manifest.Name, err)
	}
	p.running = true

	// Send initialize
	_, err = p.Call(context.Background(), "initialize", json.RawMessage(`{}`))
	if err != nil {
		p.Stop()
		return fmt.Errorf("initialize plugin %q: %w", p.manifest.Name, err)
	}

	return nil
}

// Stop shuts down the plugin subprocess.
func (p *PluginProcess) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return nil
	}

	// Send shutdown request (best-effort)
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      p.nextID,
		Method:  "shutdown",
	}
	p.nextID++
	data, _ := json.Marshal(req)
	data = append(data, '\n')
	p.stdin.Write(data)
	p.stdin.Close()

	p.running = false
	return p.cmd.Wait()
}

// Call sends a JSON-RPC request and waits for the response.
func (p *PluginProcess) Call(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return nil, fmt.Errorf("plugin %q not running", p.manifest.Name)
	}

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      p.nextID,
		Method:  method,
		Params:  params,
	}
	p.nextID++

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	data = append(data, '\n')

	if _, err := p.stdin.Write(data); err != nil {
		return nil, fmt.Errorf("write to plugin: %w", err)
	}

	// Read response line
	if !p.scanner.Scan() {
		if err := p.scanner.Err(); err != nil {
			return nil, fmt.Errorf("read from plugin: %w", err)
		}
		return nil, fmt.Errorf("plugin closed stdout")
	}

	var resp JSONRPCResponse
	if err := json.Unmarshal(p.scanner.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("plugin error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	return resp.Result, nil
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
