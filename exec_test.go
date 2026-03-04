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

func TestShellContainerNotReady(t *testing.T) {
	m := initialModel()
	m = resize(m, 80, 24)
	// containerReady is false by default

	m = typeString(m, "/shell")
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

	// Should stay in chat mode
	if m.mode != modeChat {
		t.Errorf("mode = %d, want modeChat when container not ready", m.mode)
	}

	// No cmd should be returned
	if cmd != nil {
		t.Error("should not return cmd when container not ready")
	}
}

func TestShellContainerError(t *testing.T) {
	m := initialModel()
	m = resize(m, 80, 24)

	// Simulate container error from startup
	result, _ := m.Update(containerErrMsg{err: &ContainerError{Code: ErrDockerNotFound, Message: "not found"}})
	m = result.(model)

	m = typeString(m, "/shell")
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

	// Should stay in chat mode
	if m.mode != modeChat {
		t.Errorf("mode = %d, want modeChat when container has error", m.mode)
	}

	if cmd != nil {
		t.Error("should not return cmd when container has error")
	}
}

func TestShellReturnsCmd(t *testing.T) {
	m := modelWithContainer(t, "", "", 0)

	m = typeString(m, "/shell")
	result, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = result.(model)

	// enterShellMode should return a tea.Cmd (ExecProcess)
	if cmd == nil {
		t.Error("enterShellMode should return a cmd for tea.ExecProcess")
	}

	// Should stay in chat mode (TUI suspends, not a mode switch)
	if m.mode != modeChat {
		t.Errorf("mode = %d, want modeChat (shell uses ExecProcess, not mode switch)", m.mode)
	}
}

func TestShellExitMsgSuccess(t *testing.T) {
	m := modelWithContainer(t, "", "", 0)

	result, _ := m.Update(shellExitMsg{err: nil})
	m = result.(model)

	foundInfo := false
	for _, msg := range m.messages {
		if msg.kind == msgInfo && strings.Contains(msg.content, "Shell session ended") {
			foundInfo = true
			break
		}
	}
	if !foundInfo {
		t.Errorf("should show shell ended message, messages: %+v", m.messages)
	}
}

func TestShellExitMsgError(t *testing.T) {
	m := modelWithContainer(t, "", "", 0)

	result, _ := m.Update(shellExitMsg{err: &ContainerError{Code: ErrExecFailed, Message: "connection lost"}})
	m = result.(model)

	foundErr := false
	for _, msg := range m.messages {
		if msg.kind == msgError && strings.Contains(msg.content, "connection lost") {
			foundErr = true
			break
		}
	}
	if !foundErr {
		t.Errorf("should show shell error message, messages: %+v", m.messages)
	}
}

func TestShellAutocomplete(t *testing.T) {
	m := initialModel()
	m = resize(m, 80, 24)

	m = typeString(m, "/sh")
	matches := m.autocompleteMatches()
	found := false
	for _, match := range matches {
		if match == "/shell" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("autocompleteMatches = %v, should contain /shell", matches)
	}
}

func TestAutocompleteEnterTriggersFirstMatch(t *testing.T) {
	m := modelWithContainer(t, "", "", 0)

	// Type partial command "/sh" and press Enter — should resolve to "/shell"
	m = typeString(m, "/sh")
	result, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = result.(model)

	// Should return a cmd (ExecProcess from /shell)
	if cmd == nil {
		t.Error("Enter on '/sh' should trigger /shell and return ExecProcess cmd")
	}
}

func TestContainerIDGetter(t *testing.T) {
	orig := dockerCommand
	t.Cleanup(func() { dockerCommand = orig })

	dockerCommand = fakeDockerCommand(func(args []string) (string, string, int) {
		if len(args) >= 2 && args[1] == "run" {
			return "test-container-123\n", "", 0
		}
		return "", "", 0
	})

	c := NewContainerClient(ContainerConfig{Image: "alpine:latest"})
	if err := c.Start("/workspace", nil); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if got := c.ContainerID(); got != "test-container-123" {
		t.Errorf("ContainerID() = %q, want %q", got, "test-container-123")
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
