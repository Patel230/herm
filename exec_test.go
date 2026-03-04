package main

import (
	"bufio"
	"encoding/json"
	"net"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// mockContainerClient creates a ContainerClient connected to a mock socket
// server that returns the given stdout/stderr/exitCode for any exec call.
func mockContainerClient(t *testing.T, stdout, stderr string, exitCode int) (*ContainerClient, func()) {
	t.Helper()
	sockPath, cleanup := mockServer(t, func(req jsonRPCRequest) jsonRPCResponse {
		switch req.Method {
		case "container.exec":
			result, _ := json.Marshal(CommandResult{
				Stdout:   stdout,
				Stderr:   stderr,
				ExitCode: exitCode,
			})
			return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: result}
		case "container.stop":
			result, _ := json.Marshal(map[string]string{"status": "stopped"})
			return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: result}
		default:
			return jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &jsonRPCError{Code: -32601, Message: "method not found"}}
		}
	})

	c := &ContainerClient{config: ContainerConfig{}}
	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	c.conn = conn
	c.reader = bufio.NewReader(conn)
	c.running = true

	return c, func() { c.Stop(); cleanup() }
}

// modelWithContainer creates a ready model with a mock container.
func modelWithContainer(t *testing.T, stdout, stderr string, exitCode int) (model, func()) {
	t.Helper()
	client, cleanup := mockContainerClient(t, stdout, stderr, exitCode)
	m := initialModel()
	m = resize(m, 80, 24)
	m.container = client
	m.containerReady = true
	m.worktreePath = "/tmp/test-worktree"
	return m, cleanup
}

func TestExecEchoHello(t *testing.T) {
	m, cleanup := modelWithContainer(t, "hello\n", "", 0)
	defer cleanup()

	// Type /exec echo hello and send
	m = typeString(m, "/exec echo hello")
	result, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = result.(model)

	// Should show the command being run
	found := false
	for _, msg := range m.messages {
		if msg.content == "$ echo hello" {
			found = true
			break
		}
	}
	if !found {
		t.Error("should show '$ echo hello' message")
	}

	// Execute the async cmd to get the result
	if cmd == nil {
		t.Fatal("expected non-nil cmd for async exec")
	}
	msg := cmd()

	// Feed the result back
	result, _ = m.Update(msg)
	m = result.(model)

	// Should have the output message
	foundOutput := false
	for _, msg := range m.messages {
		if msg.kind == msgSuccess && strings.Contains(msg.content, "hello") {
			foundOutput = true
			break
		}
	}
	if !foundOutput {
		t.Errorf("should show exec output 'hello', messages: %+v", m.messages)
	}
}

func TestExecNonZeroExit(t *testing.T) {
	m, cleanup := modelWithContainer(t, "", "file not found\n", 1)
	defer cleanup()

	m = typeString(m, "/exec cat missing.txt")
	result, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = result.(model)

	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	msg := cmd()

	result, _ = m.Update(msg)
	m = result.(model)

	// Should show error-style output with exit code
	foundError := false
	for _, msg := range m.messages {
		if msg.kind == msgError && strings.Contains(msg.content, "exit 1") {
			foundError = true
			break
		}
	}
	if !foundError {
		t.Errorf("should show error with exit code, messages: %+v", m.messages)
	}
}

func TestExecContainerNotReady(t *testing.T) {
	m := initialModel()
	m = resize(m, 80, 24)
	// containerReady is false by default

	m = typeString(m, "/exec echo hello")
	result, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = result.(model)

	// Should show info message about container starting
	foundInfo := false
	for _, msg := range m.messages {
		if msg.kind == msgInfo && strings.Contains(msg.content, "starting") {
			foundInfo = true
			break
		}
	}
	if !foundInfo {
		t.Errorf("should show container starting message, messages: %+v", m.messages)
	}

	// No async cmd should be returned
	if cmd != nil {
		t.Error("should not return cmd when container not ready")
	}
}

func TestExecContainerError(t *testing.T) {
	m := initialModel()
	m = resize(m, 80, 24)

	// Simulate container error from startup
	result, _ := m.Update(containerErrMsg{err: &ContainerError{Code: ErrBinaryNotFound, Message: "not found"}})
	m = result.(model)

	m = typeString(m, "/exec echo hello")
	result, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = result.(model)

	// Should show container error
	foundErr := false
	for _, msg := range m.messages {
		if msg.kind == msgError && strings.Contains(msg.content, "not found") {
			foundErr = true
			break
		}
	}
	if !foundErr {
		t.Errorf("should show container error, messages: %+v", m.messages)
	}

	if cmd != nil {
		t.Error("should not return cmd when container has error")
	}
}

func TestExecNoCommand(t *testing.T) {
	m, cleanup := modelWithContainer(t, "", "", 0)
	defer cleanup()

	m = typeString(m, "/exec")
	result, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = result.(model)

	foundUsage := false
	for _, msg := range m.messages {
		if msg.kind == msgError && strings.Contains(msg.content, "Usage") {
			foundUsage = true
			break
		}
	}
	if !foundUsage {
		t.Errorf("should show usage message, messages: %+v", m.messages)
	}
}

func TestExecAutocomplete(t *testing.T) {
	m := initialModel()
	m = resize(m, 80, 24)

	m = typeString(m, "/ex")
	matches := m.autocompleteMatches()
	if len(matches) != 1 || matches[0] != "/exec" {
		t.Errorf("autocompleteMatches = %v, want [/exec]", matches)
	}
}

func TestShutdownCleanup(t *testing.T) {
	m, cleanup := modelWithContainer(t, "", "", 0)
	defer cleanup()

	// Verify container is running
	if !m.containerReady {
		t.Fatal("container should be ready")
	}

	// Simulate ctrl+c — calls cleanup() then tea.Quit
	m.cleanup()

	// Container should be stopped
	if m.container.running {
		t.Error("container should be stopped after cleanup")
	}
}

func TestContainerReadyMsg(t *testing.T) {
	m := initialModel()
	m = resize(m, 80, 24)

	if m.containerReady {
		t.Fatal("should not be ready initially")
	}

	client, cleanup := mockContainerClient(t, "", "", 0)
	defer cleanup()

	result, _ := m.Update(containerReadyMsg{client: client, worktreePath: "/tmp/test-wt"})
	m = result.(model)

	if !m.containerReady {
		t.Error("should be ready after containerReadyMsg")
	}
	if m.container != client {
		t.Error("container should be set")
	}
	if m.worktreePath != "/tmp/test-wt" {
		t.Errorf("worktreePath = %q, want /tmp/test-wt", m.worktreePath)
	}
}
