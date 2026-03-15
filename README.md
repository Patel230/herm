# herm

A coding agent CLI that's containerized by default. Every command runs inside a Docker sandbox — nothing touches your host. No approval prompts, no "are you sure?" dialogs. Just let it work.

![demo](img/demo.gif)

## Why herm?

**Containerized by default** — The agent runs inside Docker containers with full control: installing packages, editing files, running builds. Your host machine stays untouched. No permission prompts, ever.

**Multi-provider** — Use Anthropic, OpenAI, Gemini, or Grok. Switch models on the fly.

**Self-building dev environments** — Need Python but it's not installed? herm will extend its own container by writing Dockerfiles dynamically. Dev environments are scoped per project (git repo) and persist across sessions.

**100% open-source** — Everything is open, including the system prompts. No hidden instructions, no black boxes. Read them, fork them, change them.

## Requirements

- macOS (only platform tested so far)
- Docker installed and running
- Go 1.25+

## Quick Start

```sh
go build -o herm ./cmd/herm
./herm
```

## Roadmap

- OCI container support without Docker, using Apple's [Containerization framework](https://developer.apple.com/documentation/containerization)

## Project Structure

```
herm/
├── cmd/
│   ├── herm/                  Main application
│   │   ├── prompts/           System prompt templates (embedded)
│   │   └── dockerfiles/       Container definitions (embedded)
│   └── debug/                 Debug utilities
├── .herm/
│   └── skills/                Skill definitions (e.g. devenv)
├── plans/                     Project planning docs
├── go.mod
├── LICENSE
└── README.md
```

## Test

```sh
go test ./...
```

## FAQ

<details>
<summary>How is it different from Claude Code?</summary>

Claude Code runs directly on your host and needs your approval for every potentially dangerous action. herm runs everything in containers, so the agent can act freely without risking your system. herm also supports multiple model providers and ships its system prompts in the open.
</details>

<details>
<summary>How is it different from OpenCode?</summary>

OpenCode is a great terminal AI assistant, but it runs on your host like most coding agents. herm's core idea is that containerization should be the default — not an afterthought. If the agent can't break anything, you don't need permission prompts.
</details>

<details>
<summary>How is it different from Pi Coding Agent?</summary>

<a href="https://github.com/badlogic/pi-mono">Pi</a> focuses on extensibility through TypeScript plugins and a large ecosystem of community packages. herm takes a different bet: safety through containerization. Instead of asking users to manage permissions, herm sandboxes everything by default so the agent can operate autonomously.
</details>

## Dependencies

herm is built on top of [langdag](https://langdag.com), a Go library for managing LLM conversations as directed acyclic graphs with multi-provider support. This project originally started as a way to dogfood langdag.

## License

[MIT](LICENSE)
