package main

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// mockContainerClient creates a ContainerClient backed by fakeDockerCommand
// that returns the given stdout/stderr/exitCode for exec calls.
func mockContainerClient(t *testing.T, stdout, stderr string, exitCode int) *ContainerClient {
	t.Helper()
	orig := dockerCommand
	t.Cleanup(func() { dockerCommand = orig })

	dockerCommand = fakeDockerCommand(func(args []string) (string, string, int) {
		if len(args) >= 2 {
			switch args[1] {
			case "run":
				return "mock-container-id\n", "", 0
			case "exec":
				return stdout, stderr, exitCode
			case "stop", "rm":
				return "", "", 0
			}
		}
		return "", "unknown", 1
	})

	c := NewContainerClient(ContainerConfig{Image: "alpine:latest"})
	if err := c.Start("/workspace", []MountSpec{{
		Source:      "/workspace",
		Destination: "/workspace",
	}}); err != nil {
		t.Fatalf("mockContainerClient Start: %v", err)
	}
	return c
}

// modelWithContainer creates a ready model with a mock container.
func modelWithContainer(t *testing.T, stdout, stderr string, exitCode int) model {
	t.Helper()
	client := mockContainerClient(t, stdout, stderr, exitCode)
	m := initialModel()
	m = resize(m, 80, 24)
	m.container = client
	m.containerReady = true
	m.worktreePath = "/tmp/test-worktree"
	return m
}

func TestExecEchoHello(t *testing.T) {
	m := modelWithContainer(t, "hello\n", "", 0)

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
	m := modelWithContainer(t, "", "file not found\n", 1)

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
	result, _ := m.Update(containerErrMsg{err: &ContainerError{Code: ErrDockerNotFound, Message: "not found"}})
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
	m := modelWithContainer(t, "", "", 0)

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
	m := modelWithContainer(t, "", "", 0)

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

	client := &ContainerClient{
		config:  ContainerConfig{Image: "alpine:latest"},
		running: true,
	}

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
