package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// Container error codes.
const (
	ErrBinaryNotFound = "BinaryNotFound"
	ErrImageNotFound  = "ImageNotFound"
	ErrSpawnFailed    = "SpawnFailed"
	ErrSocket         = "Socket"
	ErrProtocol       = "Protocol"
	ErrService        = "Service"
	ErrTimeout        = "Timeout"
)

// ContainerError is a typed error from the container client.
type ContainerError struct {
	Code    string
	Message string
}

func (e *ContainerError) Error() string {
	return fmt.Sprintf("container %s: %s", e.Code, e.Message)
}

// ContainerConfig holds paths needed to spawn the container service.
type ContainerConfig struct {
	ServiceBinary string // path to container-service binary
	ImagePath     string // path to OCI image
	SocketDir     string // directory for Unix sockets (temp dir if empty)
}

// MountSpec describes a filesystem mount into the container.
type MountSpec struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	ReadOnly    bool   `json:"read_only"`
}

// CommandResult holds the output of a command executed in the container.
type CommandResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

// ContainerStatus holds the current state of the container.
type ContainerStatus struct {
	State  string `json:"state"`
	Uptime int    `json:"uptime"`
}

// JSON-RPC 2.0 request/response types (private).

type jsonRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      int              `json:"id"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   *jsonRPCError    `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// JSON-RPC method parameter types.

type startParams struct {
	Workspace string      `json:"workspace"`
	Mounts    []MountSpec `json:"mounts,omitempty"`
}

type execParams struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

// ContainerClient manages a single container service subprocess and communicates
// with it over a Unix socket using JSON-RPC 2.0.
type ContainerClient struct {
	config    ContainerConfig
	socketDir string
	sockPath  string
	cmd       *exec.Cmd
	conn      net.Conn
	reader    *bufio.Reader
	mu        sync.Mutex
	nextID    atomic.Int64
	running   bool
}

// NewContainerClient creates a new client with the given config.
func NewContainerClient(config ContainerConfig) *ContainerClient {
	return &ContainerClient{config: config}
}

// IsAvailable returns true if the service binary and image exist on disk.
func (c *ContainerClient) IsAvailable() bool {
	if _, err := os.Stat(c.config.ServiceBinary); err != nil {
		return false
	}
	if _, err := os.Stat(c.config.ImagePath); err != nil {
		return false
	}
	return true
}

// Start spawns the container-service subprocess and sends a container.start
// request with the given workspace and mounts. It polls for the socket to
// appear for up to 30 seconds.
func (c *ContainerClient) Start(workspace string, mounts []MountSpec) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return &ContainerError{Code: ErrSpawnFailed, Message: "container already running"}
	}

	// Validate binary and image exist.
	if _, err := os.Stat(c.config.ServiceBinary); err != nil {
		return &ContainerError{Code: ErrBinaryNotFound, Message: c.config.ServiceBinary + " not found"}
	}
	if _, err := os.Stat(c.config.ImagePath); err != nil {
		return &ContainerError{Code: ErrImageNotFound, Message: c.config.ImagePath + " not found"}
	}

	// Set up socket path.
	socketDir := c.config.SocketDir
	if socketDir == "" {
		var err error
		socketDir, err = os.MkdirTemp("", "cpsl-sock-*")
		if err != nil {
			return &ContainerError{Code: ErrSocket, Message: fmt.Sprintf("creating socket dir: %v", err)}
		}
	}
	c.socketDir = socketDir
	c.sockPath = filepath.Join(socketDir, "container.sock")

	// Spawn subprocess.
	c.cmd = exec.Command(c.config.ServiceBinary,
		"--socket-path", c.sockPath,
		"--image-path", c.config.ImagePath,
	)
	c.cmd.Stdout = nil
	c.cmd.Stderr = nil
	if err := c.cmd.Start(); err != nil {
		return &ContainerError{Code: ErrSpawnFailed, Message: fmt.Sprintf("starting service: %v", err)}
	}

	// Poll for socket to appear.
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.Dial("unix", c.sockPath)
		if err == nil {
			c.conn = conn
			c.reader = bufio.NewReader(conn)
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if c.conn == nil {
		_ = c.cmd.Process.Kill()
		return &ContainerError{Code: ErrTimeout, Message: "timed out waiting for socket"}
	}

	// Send container.start.
	_, err := c.call("container.start", startParams{
		Workspace: workspace,
		Mounts:    mounts,
	})
	if err != nil {
		_ = c.conn.Close()
		_ = c.cmd.Process.Kill()
		return err
	}

	c.running = true
	return nil
}

// Exec runs a command inside the container and returns the result.
func (c *ContainerClient) Exec(command string, timeout int) (CommandResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return CommandResult{}, &ContainerError{Code: ErrService, Message: "container not running"}
	}

	raw, err := c.call("container.exec", execParams{
		Command: command,
		Timeout: timeout,
	})
	if err != nil {
		return CommandResult{}, err
	}

	var result CommandResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return CommandResult{}, &ContainerError{Code: ErrProtocol, Message: fmt.Sprintf("decoding exec result: %v", err)}
	}
	return result, nil
}

// Stop sends container.stop, closes the connection, and kills the subprocess.
func (c *ContainerClient) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return nil
	}

	// Best-effort stop request.
	_, _ = c.call("container.stop", nil)

	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
		_ = c.cmd.Wait()
	}

	// Clean up socket dir if we created it.
	if c.config.SocketDir == "" && c.socketDir != "" {
		_ = os.RemoveAll(c.socketDir)
	}

	c.running = false
	return nil
}

// Status queries the container's current status.
func (c *ContainerClient) Status() (ContainerStatus, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return ContainerStatus{}, &ContainerError{Code: ErrService, Message: "container not running"}
	}

	raw, err := c.call("container.status", nil)
	if err != nil {
		return ContainerStatus{}, err
	}

	var status ContainerStatus
	if err := json.Unmarshal(raw, &status); err != nil {
		return ContainerStatus{}, &ContainerError{Code: ErrProtocol, Message: fmt.Sprintf("decoding status: %v", err)}
	}
	return status, nil
}

// call sends a JSON-RPC request and reads the response. Must be called with mu held.
func (c *ContainerClient) call(method string, params interface{}) (json.RawMessage, error) {
	id := int(c.nextID.Add(1))
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, &ContainerError{Code: ErrProtocol, Message: fmt.Sprintf("marshaling request: %v", err)}
	}
	data = append(data, '\n')

	if _, err := c.conn.Write(data); err != nil {
		return nil, &ContainerError{Code: ErrSocket, Message: fmt.Sprintf("writing request: %v", err)}
	}

	line, err := c.reader.ReadBytes('\n')
	if err != nil {
		return nil, &ContainerError{Code: ErrSocket, Message: fmt.Sprintf("reading response: %v", err)}
	}

	var resp jsonRPCResponse
	if err := json.Unmarshal(line, &resp); err != nil {
		return nil, &ContainerError{Code: ErrProtocol, Message: fmt.Sprintf("decoding response: %v", err)}
	}

	if resp.Error != nil {
		return nil, &ContainerError{Code: ErrService, Message: resp.Error.Message}
	}

	return resp.Result, nil
}
