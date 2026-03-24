# herm

[![Tests](https://github.com/aduermael/herm/actions/workflows/test.yml/badge.svg)](https://github.com/aduermael/herm/actions/workflows/test.yml)
[![Prompt Length](https://github.com/aduermael/herm/actions/workflows/prompt-length.yml/badge.svg)](https://github.com/aduermael/herm/actions/workflows/prompt-length.yml)

A coding agent CLI that's containerized by default. Every command runs inside a Docker container, nothing touches your host. No approval prompts, no "are you sure?" dialogs. Just let it work.

![demo](img/demo.gif)

## Why herm?

**Containerized by default** — The agent runs inside Docker containers with full control: installing packages, editing files, running builds. Your host machine stays untouched. No permission prompts, ever.

**Multi-provider** — Use Anthropic, OpenAI, Gemini, or Grok. Switch models on the fly.

**Self-building dev environments** — Need Python but it's not installed? herm extends its own container by writing Dockerfiles dynamically. Dev environments are scoped per project (git repo) and survive container restarts — the rebuilt image persists across sessions.

**100% open-source** — Everything is open, including the system prompts. No hidden instructions, no black boxes. Read them, fork them, change them.

## Requirements

- macOS (only platform tested so far — Linux should work but hasn't been verified yet)
- Docker installed and running

## Install

### Quick install

```sh
curl -fsSL https://raw.githubusercontent.com/aduermael/herm/main/install.sh | bash
```

### Homebrew

```sh
brew tap aduermael/herm
brew install herm
```

### From source

Requires Go 1.24+.

```sh
git clone https://github.com/aduermael/herm
cd herm
go build -o herm ./cmd/herm
./herm
```

## Quick Start

```sh
herm
```

You'll need an API key for at least one provider (Anthropic, OpenAI, Grok, or Gemini) — add it via the CLI on first run.

## Roadmap

- OCI container support without Docker, using Apple's [Containerization framework](https://developer.apple.com/documentation/containerization)
- Test and verify Linux support
- Test and verify Windows support (WSL2 + Docker Desktop)

## Project Structure

```
herm/
├── cmd/
│   ├── herm/                  Main application
│   │   ├── prompts/           System prompt templates (embedded)
│   │   └── dockerfiles/       Base container definition (embedded)
│   └── debug/                 Debug utilities
├── .herm/
│   └── skills/                Skill definitions (e.g. devenv)
├── img/                       Demo assets
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

> Claude Code runs directly on your host and needs your approval for every potentially dangerous action. herm runs everything in containers, so the agent can act freely without risking your system. herm also supports multiple model providers and ships its system prompts in the open.
</details>

<details>
<summary>How is it different from OpenCode?</summary>

> OpenCode is a great terminal AI assistant, but it runs on your host like most coding agents. herm's core idea is that containerization should be the default — not an afterthought. If the agent can't break anything, you don't need permission prompts.
</details>

<details>
<summary>How is it different from Pi Coding Agent?</summary>

> [Pi](https://github.com/badlogic/pi-mono) focuses on extensibility through TypeScript plugins and a large ecosystem of community packages. herm takes a different bet: safety through containerization. Instead of asking users to manage permissions, herm sandboxes everything by default so the agent can operate autonomously.
</details>

<details>
<summary>What is the logo supposed to represent?</summary>

> It's an hermit crab called Herm, short for Herman. It represents the hermetic nature of the agent — everything sealed inside its shell.
</details>

## Dependencies

herm is built on top of [langdag](https://langdag.com), a Go library for managing LLM conversations as directed acyclic graphs with multi-provider support. This project originally started as a way to dogfood langdag.

## Community

Join the [Discord](https://discord.gg/xMEF646A) to chat, ask questions, or share feedback.

## License

[MIT](LICENSE)
