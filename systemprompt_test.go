package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"langdag.com/langdag/types"
)

// stubTool is a minimal Tool implementation for testing buildSystemPrompt.
type stubTool struct {
	name string
}

func (s stubTool) Definition() types.ToolDefinition {
	return types.ToolDefinition{
		Name:        s.name,
		Description: "stub " + s.name,
		InputSchema: json.RawMessage(`{"type":"object"}`),
	}
}

func (s stubTool) Execute(_ context.Context, _ json.RawMessage) (string, error) {
	return "", nil
}

func (s stubTool) RequiresApproval(_ json.RawMessage) bool {
	return false
}

func TestBuildSystemPromptAllTools(t *testing.T) {
	tools := []Tool{
		stubTool{"bash"},
		stubTool{"git"},
		stubTool{"devenv"},
	}
	prompt := buildSystemPrompt(tools, nil, "/workspace")

	sections := []string{
		"expert coding agent",
		"## Tools",
		"### bash",
		"### git",
		"### devenv",
		"## Practices",
		"## Communication",
		"## Environment",
		"/workspace",
	}
	for _, s := range sections {
		if !strings.Contains(prompt, s) {
			t.Errorf("prompt missing expected section/content: %q", s)
		}
	}
}

func TestBuildSystemPromptBashOnly(t *testing.T) {
	tools := []Tool{stubTool{"bash"}}
	prompt := buildSystemPrompt(tools, nil, "/work")

	if !strings.Contains(prompt, "### bash") {
		t.Error("prompt missing bash section")
	}
	if strings.Contains(prompt, "### git") {
		t.Error("prompt should not contain git section when git tool absent")
	}
	if strings.Contains(prompt, "### devenv") {
		t.Error("prompt should not contain devenv section when devenv tool absent")
	}
}

func TestBuildSystemPromptGitOnly(t *testing.T) {
	tools := []Tool{stubTool{"git"}}
	prompt := buildSystemPrompt(tools, nil, "/work")

	if !strings.Contains(prompt, "### git") {
		t.Error("prompt missing git section")
	}
	if strings.Contains(prompt, "### bash") {
		t.Error("prompt should not contain bash section when bash tool absent")
	}
}

func TestBuildSystemPromptNoTools(t *testing.T) {
	prompt := buildSystemPrompt(nil, nil, "/work")

	// Should still have the structural sections
	if !strings.Contains(prompt, "## Tools") {
		t.Error("prompt missing Tools header")
	}
	if !strings.Contains(prompt, "## Practices") {
		t.Error("prompt missing Practices section")
	}
	if !strings.Contains(prompt, "## Communication") {
		t.Error("prompt missing Communication section")
	}
	// No tool subsections
	if strings.Contains(prompt, "### bash") {
		t.Error("prompt should not contain bash section")
	}
}

func TestBuildSystemPromptWithSkills(t *testing.T) {
	skills := []Skill{
		{Name: "Testing", Description: "How to test", Content: "Write table-driven tests."},
		{Name: "Style", Description: "Code style", Content: "Use gofmt."},
	}
	prompt := buildSystemPrompt(nil, skills, "/work")

	if !strings.Contains(prompt, "## Skills") {
		t.Error("prompt missing Skills section")
	}
	if !strings.Contains(prompt, "**Testing**: How to test") {
		t.Error("prompt missing Testing skill summary")
	}
	if !strings.Contains(prompt, "**Style**: Code style") {
		t.Error("prompt missing Style skill summary")
	}
	if !strings.Contains(prompt, "### Testing") {
		t.Error("prompt missing Testing skill content section")
	}
	if !strings.Contains(prompt, "Write table-driven tests.") {
		t.Error("prompt missing Testing skill content body")
	}
	if !strings.Contains(prompt, "### Style") {
		t.Error("prompt missing Style skill content section")
	}
}

func TestBuildSystemPromptNoSkills(t *testing.T) {
	prompt := buildSystemPrompt(nil, nil, "/work")

	if strings.Contains(prompt, "## Skills") {
		t.Error("prompt should not contain Skills section when no skills loaded")
	}
}

func TestBuildSystemPromptEnvironment(t *testing.T) {
	prompt := buildSystemPrompt(nil, nil, "/my/project")

	if !strings.Contains(prompt, "/my/project") {
		t.Error("prompt missing working directory")
	}
	if !strings.Contains(prompt, "Date:") {
		t.Error("prompt missing date")
	}
}
