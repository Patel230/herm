# cpsl

A terminal-based chat interface for LLM agents with Docker container support, git worktree management, and a custom raw-terminal TUI engine.

## Features

- Interactive chat with LLM agents via [langdag](https://langdag.com)
- Docker container integration for sandboxed code execution
- Git worktree management
- Markdown rendering in the terminal
- Configurable models, skills, and system prompts
- Conversation history and scratchpad

## Build

Requires Go 1.25+.

```sh
go build -o cpsl .
```

Additional commands:

```sh
go build ./simple-chat   # minimal chat client
go build ./debug          # debug utilities
```

## Run

```sh
./cpsl
```

## Test

```sh
go test ./...
```

## Project Structure

```
*.go              Main application source (package main)
dockerfiles/      Dockerfiles for container support
simple-chat/      Minimal chat client
debug/            Debug utilities
plans/            Project planning docs
```

## License

[MIT](LICENSE)
