{{/* system_subagent: sub-agent entry point. Chains role_subagent, tools, practices, environment. Omits communication, personality, skills. */}}
{{define "system_subagent" -}}
{{- template "role_subagent" .}}{{template "tools" .}}{{template "practices" .}}{{template "environment" .}}
{{- end}}