{{/* skills: project-specific skill content loaded from .herm/skills/. Used by system.md only. */}}
{{define "skills"}}{{if .Skills}}

## Skills

{{range .Skills}}- **{{.Name}}**: {{.Description}}
{{end}}{{range .Skills}}
### {{.Name}}

{{.Content}}
{{end}}{{end}}{{end}}