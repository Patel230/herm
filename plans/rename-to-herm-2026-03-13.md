# Rename Project: cpsl → herm

**Goal:** Rename the entire project from "cpsl" to "herm" — module, binary, config directories, Docker naming, env vars, docs, tests. User will rename the GitHub repo and manually port global `~/.cpsl/` settings afterward.

**Repo description (for GitHub):** `Terminal-native AI agent with containers, worktrees, and a raw TUI engine`

---

## Phase 1: Core Constants and Config Paths

All config path logic flows from a single constant (`configDir` in config.go) and scattered hardcoded `.cpsl` strings. This phase updates the source of truth and all path references in non-test code.

- [ ] 1a: Rename `const configDir = ".cpsl"` → `".herm"` in config.go; rename `const lockFileName = ".cpsl-lock"` → `".herm-lock"` in worktree.go; update all `.cpsl` path references and comments in config.go and worktree.go
- [ ] 1b: Update all `.cpsl` path references in main.go — config dirs, attachments, tmp, skills, worktree detection, debug log path (`~/.cpsl-debug.log` → `~/.herm-debug.log`), and the `CPSL_DEBUG` env var → `HERM_DEBUG`; update the `[CPSL %s -> %s]` debug format string → `[HERM %s -> %s]`
- [ ] 1c: Update `.cpsl` path references in agent.go (`~/.cpsl` db dir), models.go (`~/.cpsl/model_catalog.json`), history.go (`.cpsl/history`), and tools.go (`.cpsl/` dir references, Dockerfile paths, comments, `cpslDir` variable → `hermDir`)

## Phase 2: Docker and Git Naming Conventions

Container names, image tags, and worktree branch prefixes all use `cpsl-` as a prefix.

- [ ] 2a: Update Docker container naming in container.go (`"cpsl-%s"` → `"herm-%s"`); update Docker image naming in tools.go (`"cpsl-local:"` and `"cpsl-"` prefix → `"herm-local:"` / `"herm-"`); update image naming in main.go (`"cpsl-local:"` → `"herm-local:"`)
- [ ] 2b: Update git worktree branch prefix in worktree.go (`"cpsl-"` → `"herm-"`) and main.go (`"cpsl-"` → `"herm-"`)

## Phase 3: Module, Binary, and Directory Structure

Rename the Go module, the `cmd/cpsl/` directory to `cmd/herm/`, and update build references.

- [ ] 3a: Update go.mod module name from `cpsl` to `herm`
- [ ] 3b: Rename directory `cmd/cpsl/` → `cmd/herm/` (including all contents: prompts/, dockerfiles/, skills/)
- [ ] 3c: Update .gitignore — binary name `cpsl` → `herm`, path `cmd/cpsl/cpsl` → `cmd/herm/herm`, `.cpsl/` → `.herm/`, `.cpsl-lock` → `.herm-lock`
- [ ] 3d: Update README.md — project title, description, build commands (`go build -o herm ./cmd/herm`), run commands, directory tree; use new repo description

## Phase 4: Tests

Update all test files to use the new naming. These are in `cmd/herm/` after Phase 3's rename.

- [ ] 4a: Update devenv_test.go — all `.cpsl/` paths, `cpslDir` variables, `/tmp/cpsl` paths, expected image names (`"cpsl-abcdef12:..."` → `"herm-abcdef12:..."`), Dockerfile messages
- [ ] 4b: Update worktree_test.go (`.cpsl-lock` → `.herm-lock`), model_test.go (`"cpsl-test-*"` temp dir prefix → `"herm-test-*"`, comments about `~/.cpsl/`), history_test.go (`.cpsl/history` → `.herm/history`)
- [ ] 4c: Update render_test.go and filetools_test.go — any test output/fixture strings containing `cpsl` paths (e.g., `cmd/cpsl/main.go` → `cmd/herm/main.go`, `# cpsl/cmd/cpsl` → `# herm/cmd/herm`)

## Phase 5: Local Project Config and Plans

Rename local `.cpsl/` to `.herm/` and update plan docs that reference old naming.

- [ ] 5a: Rename local `.cpsl/` directory → `.herm/` (project.json, config.json, Dockerfile, skills/, etc.)
- [ ] 5b: Update active plan files in `plans/` that reference `.cpsl` paths (archived plans can stay as-is — they're historical)

## Phase 6: Build Verification

- [ ] 6a: Run `go build ./cmd/herm` and `go test ./cmd/herm/...` to verify everything compiles and passes

---

**Not in scope (user handles manually):**
- Renaming the GitHub repo (`aduermael/cpsl` → `aduermael/herm`)
- Porting global `~/.cpsl/` settings to `~/.herm/`
- Updating git remote URL
