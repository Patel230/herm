{{/* personality: optional custom personality. Renders only when .Personality is set. Used by system.md only. */}}
{{define "personality"}}{{if .Personality}}

## Personality

The user has requested the following personality/tone: {{.Personality}}

Interpret this as communication style guidance — be helpful and accurate first, personality second.
{{- end}}{{end}}