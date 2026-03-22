// Package prompts embeds all system prompt templates and tool description
// markdown files. It exports the parsed template set and the tool description
// filesystem so that cmd/herm can use them without embedding files itself.
package prompts

import (
	"embed"
	"text/template"
)

//go:embed *.md
var templateFS embed.FS

// Templates is the parsed prompt template set (system, role, tools, etc.).
var Templates = template.Must(template.ParseFS(templateFS, "*.md"))

//go:embed tools/*.md
var ToolDescFS embed.FS
