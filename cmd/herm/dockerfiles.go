// dockerfiles.go embeds and resolves the default Dockerfile template.
package main

import (
	_ "embed"
	"strings"
)

// BaseDockerfile is the default Dockerfile template for new projects.
// Debian bookworm-slim with essential exploration tools: git, ripgrep, tree.
// Uses __HERM_VERSION__ placeholder, resolved at build time via resolveDockerfile.
//
//go:embed dockerfiles/base.Dockerfile
var BaseDockerfile string

// resolveDockerfile replaces the __HERM_VERSION__ placeholder in a Dockerfile
// template with the current hermImageTag.
func resolveDockerfile(content string) string {
	return strings.ReplaceAll(content, "__HERM_VERSION__", hermImageTag)
}
