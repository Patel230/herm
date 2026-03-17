{{define "environment"}}

## Environment

- Date: {{.Date}}
- Working directory: {{.WorkDir}}
- Container image: {{.ContainerImage}}
- Project mounted at: /workspace
{{- if .HasGit}}
- Git: project is in a worktree managed by herm{{if .WorktreeBranch}} (branch: {{.WorktreeBranch}}){{end}}
{{- end}}
{{- if .HasBash}}
- Attachments mounted at: /attachments (files attached to the current message are available here)
{{- end}}
{{- if or .TopLevelListing .RecentCommits .GitStatus}}

## Project context

{{- if .TopLevelListing}}

Top-level:
{{.TopLevelListing}}
{{- end}}
{{- if .RecentCommits}}

Recent commits:
{{.RecentCommits}}
{{- end}}

Uncommitted changes:
{{- if .GitStatus}}
{{.GitStatus}}
{{- else}}
clean
{{- end}}
{{- end}}
{{- end}}