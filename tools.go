package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/langdag/langdag/pkg/types"
)

// Output truncation limits for BashTool.
const (
	bashMaxLines = 200
	bashMaxBytes = 30 * 1024 // 30KB
)

// BashTool executes commands inside the Docker container via ContainerClient.
type BashTool struct {
	container *ContainerClient
	timeout   int // default timeout in seconds
}

// NewBashTool creates a BashTool with the given container client and default timeout.
func NewBashTool(container *ContainerClient, timeout int) *BashTool {
	if timeout <= 0 {
		timeout = 120
	}
	return &BashTool{container: container, timeout: timeout}
}

func (t *BashTool) Definition() types.ToolDefinition {
	return types.ToolDefinition{
		Name:        "bash",
		Description: "Execute a bash command inside the dev container at /workspace. Use this to explore files, run tests, install packages, compile code, and perform any shell operations. Commands run as root in an isolated Docker container with the project mounted at /workspace.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"command": {
					"type": "string",
					"description": "The bash command to execute"
				},
				"timeout": {
					"type": "integer",
					"description": "Timeout in seconds (default: 120)"
				}
			},
			"required": ["command"]
		}`),
	}
}

type bashInput struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
}

func (t *BashTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var in bashInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("invalid bash input: %w", err)
	}
	if in.Command == "" {
		return "", fmt.Errorf("command is required")
	}

	timeout := t.timeout
	if in.Timeout > 0 {
		timeout = in.Timeout
	}

	result, err := t.container.Exec(in.Command, timeout)
	if err != nil {
		return "", err
	}

	output := result.Stdout + result.Stderr
	output = truncateOutput(output)

	if result.ExitCode != 0 {
		return fmt.Sprintf("exit code: %d\n%s", result.ExitCode, output), nil
	}
	return output, nil
}

func (t *BashTool) RequiresApproval(_ json.RawMessage) bool {
	return false
}

// truncateOutput trims output to the last bashMaxLines lines and bashMaxBytes bytes.
func truncateOutput(s string) string {
	if len(s) <= bashMaxBytes && strings.Count(s, "\n") <= bashMaxLines {
		return s
	}

	// Truncate by bytes first.
	truncated := false
	if len(s) > bashMaxBytes {
		s = s[len(s)-bashMaxBytes:]
		truncated = true
	}

	// Truncate by lines.
	lines := strings.Split(s, "\n")
	if len(lines) > bashMaxLines {
		lines = lines[len(lines)-bashMaxLines:]
		truncated = true
	}

	result := strings.Join(lines, "\n")
	if truncated {
		result = "[output truncated, showing last portion]\n" + result
	}
	return result
}

// GitTool executes git commands on the host in the worktree directory.
type GitTool struct {
	workDir string
}

// allowedGitSubcommands is the set of git subcommands the agent may run.
var allowedGitSubcommands = map[string]bool{
	"status":   true,
	"diff":     true,
	"log":      true,
	"show":     true,
	"branch":   true,
	"checkout": true,
	"add":      true,
	"commit":   true,
	"pull":     true,
	"push":     true,
	"fetch":    true,
	"stash":    true,
	"rebase":   true,
	"merge":    true,
	"reset":    true,
	"tag":      true,
}

// NewGitTool creates a GitTool that runs in the given worktree directory.
func NewGitTool(workDir string) *GitTool {
	return &GitTool{workDir: workDir}
}

func (t *GitTool) Definition() types.ToolDefinition {
	return types.ToolDefinition{
		Name:        "git",
		Description: "Execute git commands on the host machine in the project worktree. Use this for version control operations: viewing status/diff/log, staging changes, committing, branching, pushing, etc. The `push` subcommand requires user approval.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"subcommand": {
					"type": "string",
					"description": "The git subcommand (e.g. status, diff, add, commit, push)"
				},
				"args": {
					"type": "array",
					"items": {"type": "string"},
					"description": "Arguments to pass to the git subcommand"
				}
			},
			"required": ["subcommand"]
		}`),
	}
}

type gitInput struct {
	Subcommand string   `json:"subcommand"`
	Args       []string `json:"args,omitempty"`
}

func (t *GitTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var in gitInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("invalid git input: %w", err)
	}
	if in.Subcommand == "" {
		return "", fmt.Errorf("subcommand is required")
	}

	if !allowedGitSubcommands[in.Subcommand] {
		return "", fmt.Errorf("git subcommand %q is not allowed", in.Subcommand)
	}

	args := append([]string{in.Subcommand}, in.Args...)
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = t.workDir

	out, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Sprintf("exit code: %d\n%s", exitErr.ExitCode(), string(out)), nil
		}
		return "", fmt.Errorf("git exec: %w", err)
	}

	return string(out), nil
}

func (t *GitTool) RequiresApproval(input json.RawMessage) bool {
	var in gitInput
	if err := json.Unmarshal(input, &in); err != nil {
		return false
	}
	return in.Subcommand == "push"
}
