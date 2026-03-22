{{define "system" -}}
{{- template "role" .}}{{template "tools" .}}{{template "practices" .}}{{if not .IsSubAgent}}{{template "communication" .}}{{template "personality" .}}{{template "skills" .}}{{end}}{{template "environment" .}}
{{- end}}