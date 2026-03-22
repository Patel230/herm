{{define "system_subagent" -}}
{{- template "role" .}}{{template "tools" .}}{{template "practices" .}}{{template "environment" .}}
{{- end}}