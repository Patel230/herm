# herm

A coding agent CLI that's containerized by default. Every command runs inside a Docker sandbox — nothing touches your host. No approval prompts, no "are you sure?" dialogs. Just let it work.

![demo](img/demo.gif)

## Why herm?

**Containerized by default** — The agent runs inside Docker containers with full control: installing packages, editing files, running builds. Your host machine stays untouched. No permission prompts, ever.

**Multi-provider** — Use Anthropic, OpenAI, Gemini, or Grok. Switch models on the fly.

**100% open-source** — Everything is open, including the system prompts. No hidden instructions, no black boxes. Read them, fork them, change them.

## Quick Start

Requires Go 1.25+ and Docker.

```sh
go build -o herm ./cmd/herm
./herm
```

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

## Dependencies

herm is built on top of [langdag](https://langdag.com), a Go library for managing LLM conversations as directed acyclic graphs with multi-provider support. This project originally started as a way to dogfood langdag.

## License

[MIT](LICENSE)
