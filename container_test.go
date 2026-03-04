package main

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
)

// mockServer creates a Unix socket server that handles JSON-RPC requests
// using the provided handler function. Returns the socket path and a
// cleanup function.
func mockServer(t *testing.T, handler func(req jsonRPCRequest) jsonRPCResponse) (string, func()) {
	t.Helper()
	// Use /tmp directly to keep socket path under 104 bytes (macOS limit).
	dir, err := os.MkdirTemp("/tmp", "cpsl-t-*")
	if err != nil {
		t.Fatalf("mkdirtemp: %v", err)
	}
	sockPath := filepath.Join(dir, "s.sock")

	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			var req jsonRPCRequest
			if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
				continue
			}
			resp := handler(req)
			data, _ := json.Marshal(resp)
			data = append(data, '\n')
			conn.Write(data)
		}
	}()

	return sockPath, func() { ln.Close(); os.RemoveAll(dir) }
}

// mockBinary creates a fake executable file and returns its path.
func mockBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "container-service")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// mockImage creates a fake OCI image file and returns its path.
func mockImage(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "oci-image")
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestContainerClient_IsAvailable(t *testing.T) {
	t.Run("both exist", func(t *testing.T) {
		c := NewContainerClient(ContainerConfig{
			ServiceBinary: mockBinary(t),
			ImagePath:     mockImage(t),
		})
		if !c.IsAvailable() {
			t.Error("expected IsAvailable to return true")
		}
	})

	t.Run("binary missing", func(t *testing.T) {
		c := NewContainerClient(ContainerConfig{
			ServiceBinary: "/nonexistent/binary",
			ImagePath:     mockImage(t),
		})
		if c.IsAvailable() {
			t.Error("expected IsAvailable to return false")
		}
	})

	t.Run("image missing", func(t *testing.T) {
		c := NewContainerClient(ContainerConfig{
			ServiceBinary: mockBinary(t),
			ImagePath:     "/nonexistent/image",
		})
		if c.IsAvailable() {
			t.Error("expected IsAvailable to return false")
		}
	})
}

func TestContainerClient_StartBinaryNotFound(t *testing.T) {
	c := NewContainerClient(ContainerConfig{
		ServiceBinary: "/nonexistent/binary",
		ImagePath:     mockImage(t),
	})
	err := c.Start("/workspace", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	cerr, ok := err.(*ContainerError)
	if !ok {
		t.Fatalf("expected ContainerError, got %T", err)
	}
	if cerr.Code != ErrBinaryNotFound {
		t.Errorf("expected code %s, got %s", ErrBinaryNotFound, cerr.Code)
	}
}

func TestContainerClient_StartImageNotFound(t *testing.T) {
	c := NewContainerClient(ContainerConfig{
		ServiceBinary: mockBinary(t),
		ImagePath:     "/nonexistent/image",
	})
	err := c.Start("/workspace", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	cerr, ok := err.(*ContainerError)
	if !ok {
		t.Fatalf("expected ContainerError, got %T", err)
	}
	if cerr.Code != ErrImageNotFound {
		t.Errorf("expected code %s, got %s", ErrImageNotFound, cerr.Code)
	}
}

// TestContainerClient_ExecLifecycle tests the full exec/stop lifecycle
// by connecting directly to a mock socket (bypassing subprocess spawn).
func TestContainerClient_ExecLifecycle(t *testing.T) {
	sockPath, cleanup := mockServer(t, func(req jsonRPCRequest) jsonRPCResponse {
		switch req.Method {
		case "container.exec":
			result, _ := json.Marshal(CommandResult{
				Stdout:   "hello\n",
				Stderr:   "",
				ExitCode: 0,
			})
			return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: result}
		case "container.stop":
			result, _ := json.Marshal(map[string]string{"status": "stopped"})
			return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: result}
		default:
			return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &jsonRPCError{Code: -32601, Message: "method not found"}}
		}
	})
	defer cleanup()

	// Manually set up client with existing socket (skip subprocess spawn).
	c := &ContainerClient{
		config: ContainerConfig{},
	}
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	c.conn = conn
	c.reader = bufio.NewReader(conn)
	c.running = true

	// Exec.
	result, err := c.Exec("echo hello", 120)
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if result.Stdout != "hello\n" {
		t.Errorf("expected stdout 'hello\\n', got %q", result.Stdout)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}

	// Stop.
	if err := c.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if c.running {
		t.Error("expected running to be false after stop")
	}
}

func TestContainerClient_ExecNotRunning(t *testing.T) {
	c := NewContainerClient(ContainerConfig{})
	_, err := c.Exec("echo hello", 120)
	if err == nil {
		t.Fatal("expected error")
	}
	cerr, ok := err.(*ContainerError)
	if !ok {
		t.Fatalf("expected ContainerError, got %T", err)
	}
	if cerr.Code != ErrService {
		t.Errorf("expected code %s, got %s", ErrService, cerr.Code)
	}
}

func TestContainerClient_StatusLifecycle(t *testing.T) {
	sockPath, cleanup := mockServer(t, func(req jsonRPCRequest) jsonRPCResponse {
		switch req.Method {
		case "container.status":
			result, _ := json.Marshal(ContainerStatus{State: "running", Uptime: 42})
			return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: result}
		case "container.stop":
			result, _ := json.Marshal(map[string]string{"status": "stopped"})
			return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: result}
		default:
			return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &jsonRPCError{Code: -32601, Message: "method not found"}}
		}
	})
	defer cleanup()

	c := &ContainerClient{config: ContainerConfig{}}
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	c.conn = conn
	c.reader = bufio.NewReader(conn)
	c.running = true

	status, err := c.Status()
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.State != "running" {
		t.Errorf("expected state 'running', got %q", status.State)
	}
	if status.Uptime != 42 {
		t.Errorf("expected uptime 42, got %d", status.Uptime)
	}

	_ = c.Stop()
}

func TestContainerClient_ServiceError(t *testing.T) {
	sockPath, cleanup := mockServer(t, func(req jsonRPCRequest) jsonRPCResponse {
		return jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &jsonRPCError{Code: -32000, Message: "disk full"},
		}
	})
	defer cleanup()

	c := &ContainerClient{config: ContainerConfig{}}
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	c.conn = conn
	c.reader = bufio.NewReader(conn)
	c.running = true
	defer c.Stop()

	_, execErr := c.Exec("ls", 10)
	if execErr == nil {
		t.Fatal("expected error")
	}
	cerr, ok := execErr.(*ContainerError)
	if !ok {
		t.Fatalf("expected ContainerError, got %T", execErr)
	}
	if cerr.Code != ErrService {
		t.Errorf("expected code %s, got %s", ErrService, cerr.Code)
	}
	if cerr.Message != "disk full" {
		t.Errorf("expected message 'disk full', got %q", cerr.Message)
	}
}

func TestContainerClient_StopIdempotent(t *testing.T) {
	c := NewContainerClient(ContainerConfig{})
	// Stop on a non-running client should be a no-op.
	if err := c.Stop(); err != nil {
		t.Fatalf("stop on non-running: %v", err)
	}
}

func TestContainerError_Format(t *testing.T) {
	err := &ContainerError{Code: ErrBinaryNotFound, Message: "not found"}
	expected := "container BinaryNotFound: not found"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestJSONRPC_Serialization(t *testing.T) {
	// Test request serialization.
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "container.exec",
		Params:  execParams{Command: "ls -la", Timeout: 30},
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded jsonRPCRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Method != "container.exec" {
		t.Errorf("expected method container.exec, got %s", decoded.Method)
	}
	if decoded.ID != 1 {
		t.Errorf("expected id 1, got %d", decoded.ID)
	}

	// Test response with result.
	result, _ := json.Marshal(CommandResult{Stdout: "out", ExitCode: 0})
	resp := jsonRPCResponse{JSONRPC: "2.0", ID: 1, Result: result}
	respData, _ := json.Marshal(resp)

	var decodedResp jsonRPCResponse
	if err := json.Unmarshal(respData, &decodedResp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if decodedResp.Error != nil {
		t.Error("expected no error in response")
	}

	var cr CommandResult
	if err := json.Unmarshal(decodedResp.Result, &cr); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if cr.Stdout != "out" {
		t.Errorf("expected stdout 'out', got %q", cr.Stdout)
	}

	// Test response with error.
	errResp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      2,
		Error:   &jsonRPCError{Code: -32601, Message: "method not found"},
	}
	errData, _ := json.Marshal(errResp)

	var decodedErr jsonRPCResponse
	if err := json.Unmarshal(errData, &decodedErr); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}
	if decodedErr.Error == nil {
		t.Fatal("expected error in response")
	}
	if decodedErr.Error.Message != "method not found" {
		t.Errorf("expected 'method not found', got %q", decodedErr.Error.Message)
	}
}
