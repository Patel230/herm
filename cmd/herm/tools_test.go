package main

import (
	"strings"
	"testing"
)

// --- Task 2a: truncateOutput ---

func TestTruncateOutput_Short(t *testing.T) {
	input := "hello\nworld\n"
	got := truncateOutput(input)
	if got != input {
		t.Errorf("short output should not be truncated, got %q", got)
	}
}

func TestTruncateOutput_Empty(t *testing.T) {
	got := truncateOutput("")
	if got != "" {
		t.Errorf("empty input should return empty, got %q", got)
	}
}

func TestTruncateOutput_ExactLineLimit(t *testing.T) {
	// Exactly bashMaxLines newlines — should not truncate.
	lines := make([]string, bashMaxLines+1) // 201 elements → 200 newlines when joined
	for i := range lines {
		lines[i] = "x"
	}
	input := strings.Join(lines, "\n")
	got := truncateOutput(input)
	if got != input {
		t.Error("exact line limit should not truncate")
	}
}

func TestTruncateOutput_OverLineLimit(t *testing.T) {
	// bashMaxLines + 10 newlines → should truncate.
	lines := make([]string, bashMaxLines+12)
	for i := range lines {
		lines[i] = "line"
	}
	input := strings.Join(lines, "\n")
	got := truncateOutput(input)

	if !strings.HasPrefix(got, "[output truncated, showing last portion]\n") {
		t.Error("truncated output should start with truncation notice")
	}
	// Count lines in result (after the prefix).
	resultLines := strings.Split(got, "\n")
	// First line is the notice, then bashMaxLines lines.
	if len(resultLines) > bashMaxLines+2 {
		t.Errorf("result has %d lines, want ≤ %d", len(resultLines), bashMaxLines+2)
	}
}

func TestTruncateOutput_ExactByteLimit(t *testing.T) {
	input := strings.Repeat("a", bashMaxBytes)
	got := truncateOutput(input)
	if got != input {
		t.Error("exact byte limit should not truncate")
	}
}

func TestTruncateOutput_OverByteLimit(t *testing.T) {
	input := strings.Repeat("a", bashMaxBytes+100)
	got := truncateOutput(input)

	if !strings.HasPrefix(got, "[output truncated, showing last portion]\n") {
		t.Error("byte-truncated output should start with truncation notice")
	}
	// The result (minus prefix) should be ≤ bashMaxBytes.
	body := strings.TrimPrefix(got, "[output truncated, showing last portion]\n")
	if len(body) > bashMaxBytes {
		t.Errorf("body length %d > bashMaxBytes %d", len(body), bashMaxBytes)
	}
}

func TestTruncateOutput_BothLimitsExceeded(t *testing.T) {
	// Create content that exceeds both byte and line limits.
	line := strings.Repeat("x", 200) + "\n"
	input := strings.Repeat(line, bashMaxLines+50)
	got := truncateOutput(input)

	if !strings.HasPrefix(got, "[output truncated, showing last portion]\n") {
		t.Error("expected truncation notice")
	}
}

func TestTruncateOutput_KeepsLastLines(t *testing.T) {
	// Verify that the LAST lines are kept, not the first.
	lines := make([]string, bashMaxLines+20)
	for i := range lines {
		lines[i] = "old"
	}
	// Mark the last few lines.
	lines[len(lines)-1] = "LAST"
	lines[len(lines)-2] = "SECOND_LAST"
	input := strings.Join(lines, "\n")
	got := truncateOutput(input)

	if !strings.Contains(got, "LAST") {
		t.Error("truncated output should contain the last line")
	}
	if !strings.Contains(got, "SECOND_LAST") {
		t.Error("truncated output should contain the second-to-last line")
	}
}

// --- Task 2b: gitArgsContainForce ---

func TestGitArgsContainForce_Force(t *testing.T) {
	if !gitArgsContainForce([]string{"push", "--force"}) {
		t.Error("should detect --force")
	}
}

func TestGitArgsContainForce_ShortFlag(t *testing.T) {
	if !gitArgsContainForce([]string{"push", "-f"}) {
		t.Error("should detect -f")
	}
}

func TestGitArgsContainForce_ForceWithLease(t *testing.T) {
	if !gitArgsContainForce([]string{"push", "--force-with-lease"}) {
		t.Error("should detect --force-with-lease")
	}
}

func TestGitArgsContainForce_NoForce(t *testing.T) {
	if gitArgsContainForce([]string{"push", "origin", "main"}) {
		t.Error("should not detect force in normal args")
	}
}

func TestGitArgsContainForce_Empty(t *testing.T) {
	if gitArgsContainForce(nil) {
		t.Error("nil args should return false")
	}
	if gitArgsContainForce([]string{}) {
		t.Error("empty args should return false")
	}
}

func TestGitArgsContainForce_MixedArgs(t *testing.T) {
	if !gitArgsContainForce([]string{"origin", "main", "--force", "--set-upstream"}) {
		t.Error("should detect --force among other args")
	}
}

func TestGitArgsContainForce_SimilarButNotForce(t *testing.T) {
	// "--forceful" or "-force" should not match.
	if gitArgsContainForce([]string{"--forceful"}) {
		t.Error("--forceful should not match")
	}
}

// --- Task 2c: gitCredentialHint ---

func TestGitCredentialHint_AuthenticationFailed(t *testing.T) {
	output := "fatal: Authentication failed for 'https://github.com/user/repo.git/'"
	hint := gitCredentialHint(output)
	if hint == "" {
		t.Error("should detect authentication failure")
	}
}

func TestGitCredentialHint_PermissionDenied(t *testing.T) {
	output := "git@github.com: Permission denied (publickey).\nfatal: Could not read from remote repository."
	hint := gitCredentialHint(output)
	if hint == "" {
		t.Error("should detect permission denied")
	}
}

func TestGitCredentialHint_CouldNotReadUsername(t *testing.T) {
	output := "fatal: could not read Username for 'https://github.com': terminal prompts disabled"
	hint := gitCredentialHint(output)
	if hint == "" {
		t.Error("should detect could not read username")
	}
}

func TestGitCredentialHint_PasswordAuthRemoved(t *testing.T) {
	output := "remote: Support for password authentication was removed on August 13, 2021."
	hint := gitCredentialHint(output)
	if hint == "" {
		t.Error("should detect password auth removal message")
	}
}

func TestGitCredentialHint_HostKeyVerification(t *testing.T) {
	output := "Host key verification failed.\nfatal: Could not read from remote repository."
	hint := gitCredentialHint(output)
	if hint == "" {
		t.Error("should detect host key verification failure")
	}
}

func TestGitCredentialHint_ConnectionRefused(t *testing.T) {
	hint := gitCredentialHint("ssh: connect to host github.com port 22: Connection refused")
	if hint == "" {
		t.Error("should detect connection refused")
	}
}

func TestGitCredentialHint_ConnectionTimedOut(t *testing.T) {
	hint := gitCredentialHint("ssh: connect to host github.com port 22: Connection timed out")
	if hint == "" {
		t.Error("should detect connection timed out")
	}
}

func TestGitCredentialHint_NormalOutput(t *testing.T) {
	outputs := []string{
		"Everything up-to-date",
		"To github.com:user/repo.git\n  abc123..def456  main -> main",
		"Already up to date.",
		"fatal: not a git repository",
		"error: failed to push some refs to 'origin'",
	}
	for _, o := range outputs {
		hint := gitCredentialHint(o)
		if hint != "" {
			t.Errorf("should not trigger on normal output %q, got hint: %q", o, hint)
		}
	}
}

func TestGitCredentialHint_CaseInsensitive(t *testing.T) {
	// The patterns are compared case-insensitively via strings.ToLower.
	hint := gitCredentialHint("AUTHENTICATION FAILED for repo")
	if hint == "" {
		t.Error("should detect case-insensitive match")
	}
}

func TestGitCredentialHint_HintContent(t *testing.T) {
	hint := gitCredentialHint("Permission denied (publickey)")
	if !strings.Contains(hint, "credentials") && !strings.Contains(hint, "SSH") {
		t.Errorf("hint should mention credentials/SSH, got: %q", hint)
	}
}
