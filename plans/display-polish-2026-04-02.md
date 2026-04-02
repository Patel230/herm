# Display Polish: Hide Internal Tool Calls, Fix Ordering & Empty Lines

**Goal:** Clean up four remaining display issues from the display overhaul: hide internal agent-checking tool calls, ensure the main agent status line is always last, suppress sleep/wait tool blocks, and eliminate spurious empty lines in rendered blocks.

**Context:**
- The previous display overhaul (`display-overhaul-2026-04-02.md`) established grouped tool blocks, sub-agent metrics, and an enriched status line. Four visual artifacts remain.
- The main agent uses the "agent" tool with `task: "status"` to poll background sub-agents, and uses bash `sleep` commands to wait. Both create visible but unhelpful UI blocks.
- Sub-agent display lines are appended *after* the main status line in `buildBlockRows()`, pushing the status line up from its expected bottom position.
- `strings.Split(content, "\n")` on content ending with `\n` creates trailing empty string elements that become blank lines inside rendered boxes.

**Key files:**
- `cmd/herm/agentui.go` — event handling, creates `msgToolCall`/`msgToolResult` messages (lines 366-425)
- `cmd/herm/content.go` — `toolCallSummary()` (lines 194-244), `collapseToolResult()` (lines 305-322)
- `cmd/herm/render.go` — `buildBlockRows()` (lines 300-438), status line (401-429), sub-agent lines (431-436)
- `cmd/herm/style.go` — `renderToolGroup()` (line 416), `renderToolBox()` (line 258), content splitting in both

---

## Phase 1: Hide internal "agent" status-check tool calls

When the main agent polls a background sub-agent, it calls the "agent" tool with `{task: "status", agent_id: "..."}`. This creates a `msgToolCall` (showing `~ agent`) and a `msgToolResult` with the status response. Since the status-line and sub-agent display already show this info, these tool blocks are redundant noise.

**Approach:** Filter at the event-handling layer in `agentui.go`. When `EventToolCallStart` fires for tool name `"agent"` and the input contains `"task":"status"`, skip creating the `msgToolCall` message. Track this "suppressed" state so the corresponding `EventToolResult` is also skipped. This avoids polluting the message list with internal bookkeeping.

**Edge cases:**
- Actual agent spawns (`task != "status"`) must NOT be filtered — only status polls
- The trace collector should still receive these events for debugging (don't skip `traceCollector` calls)
- The tool timer should not start for suppressed tools

- [ ] 1a: Add a `suppressedToolID` field (or set) to `App` to track tool calls that should be hidden from the UI
- [ ] 1b: In `EventToolCallStart` handler, detect agent status checks (tool name `"agent"`, input has `"task":"status"`) and add the tool ID to the suppressed set instead of appending a `msgToolCall`. Still forward to `traceCollector`
- [ ] 1c: In `EventToolResult` handler, check if the tool ID is in the suppressed set. If so, skip creating `msgToolResult` and remove from set. Still forward to `traceCollector` and count stats
- [ ] 1d: Add tests for the suppression logic: agent status check is hidden, agent spawn is visible, trace collector still receives both

## Phase 2: Hide sleep/wait bash commands used for sub-agent polling

The main agent often calls `bash` with commands like `sleep 15 && echo "done waiting"` or `sleep 30 && echo "done"` to wait for sub-agents. These show up as visible tool blocks but provide no useful information since sub-agent progress is already shown in the sub-agent display.

**Approach:** Extend the suppression mechanism from Phase 1. Detect bash tool calls where the command matches a sleep-only pattern (e.g., starts with `sleep` and contains no meaningful work beyond echo). Use the same `suppressedToolID` set.

**Edge cases:**
- Only suppress pure sleep commands (e.g., `sleep N`, `sleep N && echo "..."`)
- Do NOT suppress bash commands that contain sleep as part of a larger pipeline
- Use a simple regex/heuristic: `^\s*sleep\s+\d+\s*(&&\s*echo\s+.*)?$`

- [ ] 2a: Add an `isSleepWaitCommand(input json.RawMessage) bool` helper in `content.go` that parses the bash command and checks if it's a pure sleep/wait
- [ ] 2b: Extend the suppression check in `EventToolCallStart` to also suppress bash sleep-wait commands
- [ ] 2c: Add tests for sleep detection: pure sleep suppressed, sleep-in-pipeline not suppressed, non-sleep bash visible

## Phase 3: Move status line below sub-agent display

Currently in `buildBlockRows()` (render.go:401-436), the main agent status line is appended first (lines 401-429), then sub-agent display lines are appended after (lines 431-436). This means sub-agents push the status line up. The status line should always be the very last thing before the input area.

**Approach:** Swap the order: emit sub-agent display lines first, then the status line.

- [ ] 3a: In `buildBlockRows()`, move the sub-agent display block (lines 431-436) to before the status line block (lines 401-429)
- [ ] 3b: Add a test verifying that when both status line and sub-agent lines are present, the status line appears after the sub-agent lines in the output

## Phase 4: Fix empty lines in rendered blocks

Tool result content often ends with `\n`, and `strings.Split(content, "\n")` produces a trailing empty string that becomes a blank line inside bordered blocks. This affects `renderToolBox()`, `renderToolGroup()`, and `collapseToolResult()`.

**Approach:** Trim trailing newlines from content before splitting in the rendering functions. Apply consistently in all three locations.

**Locations to fix:**
- `style.go:416` — `renderToolGroup()`: `strings.Split(content, "\n")` for tool output
- `style.go:258` — `renderToolBox()`: `strings.Split(content, "\n")` for content lines
- `style.go:198` — `renderToolBox()`: `strings.Split(content, "\n")` for width calculation
- `content.go:306` — `collapseToolResult()`: `strings.Split(result, "\n")` — trailing empty element inflates line count

- [ ] 4a: In `collapseToolResult()`, trim trailing newlines from `result` before splitting (`strings.TrimRight(result, "\n")`)
- [ ] 4b: In `renderToolGroup()`, trim trailing newlines from `content` before the split at line 416
- [ ] 4c: In `renderToolBox()`, trim trailing newlines from `content` at the start of the function (before both split locations at lines 198 and 258)
- [ ] 4d: Add tests verifying that tool results ending with `\n` do not produce empty lines inside rendered blocks

## Phase 5: Integration tests

- [ ] 5a: Add an end-to-end render test with a mock conversation that includes: agent status checks (should be hidden), sleep waits (should be hidden), active sub-agents with status line (status line at bottom), and tool results with trailing newlines (no empty lines in blocks)
