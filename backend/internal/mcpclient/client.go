// Package mcpclient provides the Model Context Protocol (MCP) client implementation.
// This file implements the base stdio-based MCP client which manages a subprocess.
package mcpclient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

// Client is one stdio MCP client session.
type Client struct {
	server models.MCPServer

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	cancel context.CancelFunc
	done   chan struct{}

	writeMu sync.Mutex
	mu      sync.Mutex
	pending map[string]chan pendingResponse
	nextID  int64

	closeOnce sync.Once
	doneErr   error
	stopping  atomic.Bool
}

// NewClient creates a client for a configured MCP server.
func NewClient(server models.MCPServer) *Client {
	return &Client{
		server:  server,
		done:    make(chan struct{}),
		pending: make(map[string]chan pendingResponse),
	}
}

// Start launches the MCP server subprocess and performs MCP initialization.
func (c *Client) Start(ctx context.Context) error {
	if c.server.Transport != "stdio" {
		return fmt.Errorf("unsupported MCP transport %q", c.server.Transport)
	}
	if c.server.Command == nil || *c.server.Command == "" {
		return fmt.Errorf("command is required for stdio MCP server")
	}

	procCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	cmd := exec.CommandContext(procCtx, *c.server.Command, c.server.Args...)
	cmd.Env = os.Environ()
	for key, value := range c.server.Env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("open stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("open stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("open stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("start MCP server %q: %w", c.server.Name, err)
	}

	c.cmd = cmd
	c.stdin = stdin
	c.stdout = stdout
	c.stderr = stderr

	go c.readLoop()
	go c.stderrLoop()
	go c.waitLoop()

	initCtx, initCancel := context.WithTimeout(ctx, time.Duration(defaultRequestTimeout)*time.Second)
	defer initCancel()
	if err := c.initialize(initCtx); err != nil {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer stopCancel()
		_ = c.Stop(stopCtx)
		return err
	}

	return nil
}

// Stop terminates the server subprocess and fails any pending requests.
func (c *Client) Stop(ctx context.Context) error {
	if c.cancel != nil {
		c.stopping.Store(true)
		c.cancel()
	}
	if c.stdin != nil {
		_ = c.stdin.Close()
	}

	select {
	case <-c.done:
		if c.doneErr == io.EOF || c.doneErr == context.Canceled {
			return nil
		}
		return c.doneErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ListTools discovers all tools exposed by the server.
func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	var tools []Tool
	cursor := ""

	for {
		params := map[string]interface{}{}
		if cursor != "" {
			params["cursor"] = cursor
		}

		var result listToolsResult
		if err := c.request(ctx, "tools/list", params, &result); err != nil {
			return nil, err
		}
		tools = append(tools, result.Tools...)
		if result.NextCursor == "" {
			break
		}
		cursor = result.NextCursor
	}

	return tools, nil
}

// CallTool invokes a tool on the MCP server.
func (c *Client) CallTool(ctx context.Context, name string, arguments json.RawMessage) (*CallToolResult, error) {
	var args map[string]interface{}
	if len(bytes.TrimSpace(arguments)) == 0 {
		args = map[string]interface{}{}
	} else if err := json.Unmarshal(arguments, &args); err != nil {
		return nil, fmt.Errorf("arguments must be a JSON object: %w", err)
	}
	if args == nil {
		args = map[string]interface{}{}
	}

	params := map[string]interface{}{
		"name":      name,
		"arguments": args,
	}

	var result CallToolResult
	if err := c.request(ctx, "tools/call", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) initialize(ctx context.Context) error {
	params := map[string]interface{}{
		"protocolVersion": ProtocolVersion,
		"capabilities":    map[string]interface{}{},
		"clientInfo": implementation{
			Name:    "omnillm-studio",
			Version: "0.2.0",
		},
	}

	var result initializeResult
	if err := c.request(ctx, "initialize", params, &result); err != nil {
		return fmt.Errorf("initialize MCP server: %w", err)
	}
	if result.ProtocolVersion == "" {
		return fmt.Errorf("initialize MCP server: missing protocol version")
	}
	return c.notify(ctx, "notifications/initialized", nil)
}

func (c *Client) request(ctx context.Context, method string, params interface{}, out interface{}) error {
	id := atomic.AddInt64(&c.nextID, 1)
	key := fmt.Sprintf("%d", id)
	ch := make(chan pendingResponse, 1)

	c.mu.Lock()
	c.pending[key] = ch
	c.mu.Unlock()

	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}
	if err := c.writeJSON(req); err != nil {
		c.removePending(key)
		return err
	}

	select {
	case resp := <-ch:
		if resp.err != nil {
			return resp.err
		}
		if out == nil {
			return nil
		}
		if len(resp.result) == 0 {
			return nil
		}
		if err := json.Unmarshal(resp.result, out); err != nil {
			return fmt.Errorf("decode %s result: %w", method, err)
		}
		return nil
	case <-ctx.Done():
		c.removePending(key)
		return ctx.Err()
	case <-c.done:
		c.removePending(key)
		if c.doneErr != nil {
			return c.doneErr
		}
		return io.EOF
	}
}

func (c *Client) notify(ctx context.Context, method string, params interface{}) error {
	req := rpcRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	done := make(chan error, 1)
	go func() {
		done <- c.writeJSON(req)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-c.done:
		if c.doneErr != nil {
			return c.doneErr
		}
		return io.EOF
	}
}

func (c *Client) writeJSON(value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal MCP message: %w", err)
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if c.stdin == nil {
		return fmt.Errorf("MCP server stdin is not open")
	}
	if _, err := c.stdin.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write MCP message: %w", err)
	}
	return nil
}

func (c *Client) readLoop() {
	scanner := bufio.NewScanner(c.stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), maxRPCMessageBytes)

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		var resp rpcResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			log.Printf("[mcp] %s sent invalid JSON-RPC on stdout: %v", c.server.Name, err)
			continue
		}
		if len(resp.ID) == 0 {
			continue
		}
		if resp.Method != "" {
			log.Printf("[mcp] %s sent unsupported client request %q", c.server.Name, resp.Method)
			continue
		}
		c.completePending(string(resp.ID), resp)
	}

	err := scanner.Err()
	if err == nil {
		err = io.EOF
	}
	c.closeWithError(err)
}

func (c *Client) stderrLoop() {
	scanner := bufio.NewScanner(c.stderr)
	scanner.Buffer(make([]byte, 0, 8*1024), 256*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			log.Printf("[mcp:%s] %s", c.server.Name, line)
		}
	}
}

func (c *Client) waitLoop() {
	if c.cmd == nil {
		return
	}
	err := c.cmd.Wait()
	if err != nil {
		if c.stopping.Load() {
			c.closeWithError(io.EOF)
			return
		}
		c.closeWithError(fmt.Errorf("MCP server %q exited: %w", c.server.Name, err))
		return
	}
	c.closeWithError(io.EOF)
}

func (c *Client) completePending(key string, resp rpcResponse) {
	c.mu.Lock()
	ch, ok := c.pending[key]
	if ok {
		delete(c.pending, key)
	}
	c.mu.Unlock()

	if !ok {
		return
	}
	if resp.Error != nil {
		ch <- pendingResponse{err: resp.Error}
		return
	}
	ch <- pendingResponse{result: resp.Result}
}

func (c *Client) removePending(key string) {
	c.mu.Lock()
	delete(c.pending, key)
	c.mu.Unlock()
}

func (c *Client) closeWithError(err error) {
	c.closeOnce.Do(func() {
		c.doneErr = err

		c.mu.Lock()
		for key, ch := range c.pending {
			ch <- pendingResponse{err: err}
			delete(c.pending, key)
		}
		c.mu.Unlock()

		close(c.done)
	})
}
