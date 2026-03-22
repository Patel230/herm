{{/* personality: optional custom personality. Renders only when .Personality is set. Used by system.md only. */}}
{{define "personality"}}{{if .Personality}}

## Personality

{{.Personality}}
{{- end}}{{end}}