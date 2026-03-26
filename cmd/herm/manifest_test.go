package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseVersionLine_Go(t *testing.T) {
	name, ver := parseVersionLine("go version go1.22.5 linux/amd64")
	if name != "go" || ver != "1.22.5" {
		t.Errorf("got %q %q, want go 1.22.5", name, ver)
	}
}

func TestParseVersionLine_Node(t *testing.T) {
	name, ver := parseVersionLine("v22.14.0")
	if name != "node" || ver != "22.14.0" {
		t.Errorf("got %q %q, want node 22.14.0", name, ver)
	}
}

func TestParseVersionLine_Python(t *testing.T) {
	name, ver := parseVersionLine("Python 3.11.2")
	if name != "python3" || ver != "3.11.2" {
		t.Errorf("got %q %q, want python3 3.11.2", name, ver)
	}
}

func TestParseVersionLine_Ruby(t *testing.T) {
	name, ver := parseVersionLine("ruby 3.1.2p20 (2022-04-12 revision 4491bb740a) [x86_64-linux]")
	if name != "ruby" || ver != "3.1.2" {
		t.Errorf("got %q %q, want ruby 3.1.2", name, ver)
	}
}

func TestParseVersionLine_Rustc(t *testing.T) {
	name, ver := parseVersionLine("rustc 1.75.0 (82e1608df 2023-12-21)")
	if name != "rustc" || ver != "1.75.0" {
		t.Errorf("got %q %q, want rustc 1.75.0", name, ver)
	}
}

func TestParseVersionLine_Java(t *testing.T) {
	name, ver := parseVersionLine(`openjdk version "21.0.1" 2023-10-17`)
	if name != "java" || ver != "21.0.1" {
		t.Errorf("got %q %q, want java 21.0.1", name, ver)
	}
}

func TestParseVersionLine_JavaShort(t *testing.T) {
	name, ver := parseVersionLine("openjdk 21.0.1 2023-10-17")
	if name != "java" || ver != "21.0.1" {
		t.Errorf("got %q %q, want java 21.0.1", name, ver)
	}
}

func TestParseVersionLine_Unknown(t *testing.T) {
	name, ver := parseVersionLine("some random output")
	if name != "" || ver != "" {
		t.Errorf("got %q %q, want empty", name, ver)
	}
}

func TestParseManifest_Full(t *testing.T) {
	output := `=RUNTIMES=
go version go1.22.5 linux/amd64
v22.14.0
Python 3.11.2
=TOOLS=
git rg tree curl wget make
`
	got := parseManifest(output)

	if !strings.Contains(got, "Runtimes: go 1.22.5, node 22.14.0, python3 3.11.2") {
		t.Errorf("runtimes line wrong: %s", got)
	}
	if !strings.Contains(got, "System tools: git, rg, tree, curl, wget, make") {
		t.Errorf("tools line wrong: %s", got)
	}

	// Verify compactness — should be exactly 2 lines.
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d: %q", len(lines), got)
	}
}

func TestParseManifest_RuntimesOnly(t *testing.T) {
	output := "=RUNTIMES=\ngo version go1.22.5 linux/amd64\n=TOOLS=\n\n"
	got := parseManifest(output)

	if !strings.Contains(got, "Runtimes: go 1.22.5") {
		t.Errorf("expected runtimes, got: %s", got)
	}
	if strings.Contains(got, "System tools") {
		t.Error("should not have tools line when no tools detected")
	}
}

func TestParseManifest_ToolsOnly(t *testing.T) {
	output := "=RUNTIMES=\n=TOOLS=\ngit rg tree\n"
	got := parseManifest(output)

	if strings.Contains(got, "Runtimes") {
		t.Error("should not have runtimes line when none detected")
	}
	if !strings.Contains(got, "System tools: git, rg, tree") {
		t.Errorf("expected tools, got: %s", got)
	}
}

func TestParseManifest_Empty(t *testing.T) {
	output := "=RUNTIMES=\n=TOOLS=\n"
	got := parseManifest(output)
	if got != "" {
		t.Errorf("expected empty manifest, got: %q", got)
	}
}

func TestManifestPath(t *testing.T) {
	tool := NewDevEnvTool(nil, "/tmp/.herm", "/tmp", nil, "", nil, nil)
	got := tool.manifestPath()
	if got != "/tmp/.herm/environment.md" {
		t.Errorf("manifestPath() = %q, want /tmp/.herm/environment.md", got)
	}
}

func TestManifestStale_Missing(t *testing.T) {
	dir := t.TempDir()
	hermDir := filepath.Join(dir, ".herm")
	os.MkdirAll(hermDir, 0o755)

	// Dockerfile exists but no manifest.
	os.WriteFile(filepath.Join(hermDir, "Dockerfile"), []byte("FROM x\n"), 0o644)

	tool := NewDevEnvTool(nil, hermDir, dir, nil, "", nil, nil)
	if !tool.manifestStale() {
		t.Error("manifest should be stale when missing")
	}
}

func TestManifestStale_OlderThanDockerfile(t *testing.T) {
	dir := t.TempDir()
	hermDir := filepath.Join(dir, ".herm")
	os.MkdirAll(hermDir, 0o755)

	// Write manifest first, then Dockerfile (newer).
	mPath := filepath.Join(hermDir, manifestFile)
	os.WriteFile(mPath, []byte("Runtimes: go 1.22\n"), 0o644)
	// Ensure Dockerfile has a later mtime.
	time.Sleep(10 * time.Millisecond)
	os.WriteFile(filepath.Join(hermDir, "Dockerfile"), []byte("FROM x\n"), 0o644)

	tool := NewDevEnvTool(nil, hermDir, dir, nil, "", nil, nil)
	if !tool.manifestStale() {
		t.Error("manifest should be stale when older than Dockerfile")
	}
}

func TestManifestStale_Fresh(t *testing.T) {
	dir := t.TempDir()
	hermDir := filepath.Join(dir, ".herm")
	os.MkdirAll(hermDir, 0o755)

	// Dockerfile first, then manifest (newer).
	os.WriteFile(filepath.Join(hermDir, "Dockerfile"), []byte("FROM x\n"), 0o644)
	time.Sleep(10 * time.Millisecond)
	os.WriteFile(filepath.Join(hermDir, manifestFile), []byte("Runtimes: go 1.22\n"), 0o644)

	tool := NewDevEnvTool(nil, hermDir, dir, nil, "", nil, nil)
	if tool.manifestStale() {
		t.Error("manifest should not be stale when newer than Dockerfile")
	}
}

func TestManifestStale_NoDockerfile(t *testing.T) {
	dir := t.TempDir()
	hermDir := filepath.Join(dir, ".herm")
	os.MkdirAll(hermDir, 0o755)

	// Only manifest, no Dockerfile.
	os.WriteFile(filepath.Join(hermDir, manifestFile), []byte("Runtimes: go 1.22\n"), 0o644)

	tool := NewDevEnvTool(nil, hermDir, dir, nil, "", nil, nil)
	if tool.manifestStale() {
		t.Error("manifest should not be stale when no Dockerfile exists")
	}
}

func TestGenerateManifest_NilContainer(t *testing.T) {
	dir := t.TempDir()
	hermDir := filepath.Join(dir, ".herm")

	tool := NewDevEnvTool(nil, hermDir, dir, nil, "", nil, nil)
	if err := tool.generateManifest(); err != nil {
		t.Errorf("generateManifest with nil container should return nil, got: %v", err)
	}
}
