{{/* system: main agent entry point. Chains role, tools, practices, communication, personality, skills, environment. */}}
{{define "system" -}}
{{- template "role" .}}{{template "tools" .}}{{template "practices" .}}{{template "communication" .}}{{template "personality" .}}{{template "skills" .}}{{template "environment" .}}
{{- end}}