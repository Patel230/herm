# Fix Sub-Agent Display Bugs: Timer, Ordering, and Mode Default

**Goal:** Fix three bugs observed in the sub-agent display after the lifecycle fixes in the previous plan (`fix-subagent-lifecycle-and-display-2026-04-02.md`).

**Context:**

Observed in `debug-20260402-223330.json`: user spawned 3 background explore agents. Agents completed in 9.3s, 28.5s, and 77.7s respectively, but the display showed all three with identical elapsed times (~1236s) and the times kept ticking after completion. The main agent's response text appeared above the sub-agent status section, breaking chronological order. A fourth tool call omitted the `mode` field, producing a visible error box.

**Bugs:**

1. **Sub-agent timers never freeze on completion.** `formatSubAgentLine()` (`render.go:652-659`) has identical logic for done and not-done agents: both branches compute `elapsed = time.Since(sa.startTime)`. The `completedAt` field is correctly set in `EventSubAgentStatus` handler (`agentui.go:573`) but the rendering code ignores it. All completed agents show the same ever-increasing time instead of their actual durations.

2. **Main agent text renders above sub-agent display.** `buildBlockRows()` (`render.go:407-432`) renders in this order: messages → streaming text → sub-agent lines → status line. When the main agent streams its response (after `backgroundCompletion` injects results), the text appears above the sub-agent section. The user sees the response growing above still-visible sub-agent status lines. Chronologically, sub-agents started before the response, so they should appear above it.

3. **Empty mode on agent resume causes error.** The LLM called `{"agent_id": "e6713a08", "task": "give me your summary"}` without a `mode` field. The status-check bypass (`subagent.go:359-361`) only activates when `task == "status"`. For any other task with `agent_id`, mode validation fails with the error `mode must be "explore" or "implement", got ""`. When resuming an existing agent, the mode should default to `"explore"` (cheap model, read-only tools — safe default) rather than failing.

**Key files:**
- `cmd/herm/render.go` — `formatSubAgentLine()` (line 633), `buildBlockRows()` (line 324), `subAgentDisplay` struct (line 488)
- `cmd/herm/subagent.go` — `Execute()` (line 349), mode validation (line 364), tool definition schema (line 100)
- `cmd/herm/agentui.go` — `EventSubAgentStatus` handler (line 569)

---

## Phase 1: Fix sub-agent elapsed time to freeze on completion

The `formatSubAgentLine` function has a done/not-done branch (lines 652-659) where both branches are identical — a copy-paste bug. The done branch should use `sa.completedAt.Sub(sa.startTime)` to show the frozen elapsed time at the moment of completion.

- [ ] 1a: In `formatSubAgentLine()` (`render.go:656`), change the done branch from `elapsed = time.Since(sa.startTime)` to `elapsed = sa.completedAt.Sub(sa.startTime)`. Add a guard: if `completedAt` is zero (shouldn't happen, but defensive), fall back to `time.Since(sa.startTime)`
- [ ] 1b: Add test: create two `subAgentDisplay` entries with different `startTime` and `completedAt` values (both `done: true`), call `formatSubAgentLine()` on each, verify the elapsed times differ and match the expected `completedAt - startTime` durations

## Phase 2: Render sub-agent display above streaming text

In `buildBlockRows()`, swap the rendering order so sub-agent display lines appear before streaming text. New order: messages → sub-agent lines → streaming text → status line. This ensures that sub-agent activity (which started first) appears above the main agent's response text (which is generated after agents complete).

- [ ] 2a: In `buildBlockRows()` (`render.go:407-432`), move the sub-agent display block (lines 427-432) to render before the streaming text block (lines 408-424). The status line section (lines 433-462) stays at the bottom unchanged
- [ ] 2b: Add test: set up an `App` with both `streamingText` and populated `subAgents`, call `buildBlockRows()`, verify that sub-agent display lines appear before the streaming text lines in the output

## Phase 3: Default mode to "explore" when resuming with agent_id

When `agent_id` is provided and `mode` is empty, default to `"explore"` instead of returning an error. This handles resume-with-query patterns where the LLM provides `agent_id` + `task` but omits `mode`.

Two changes: relax the tool definition schema (remove mode from required array since it's only required for new spawns), and add defaulting logic in `Execute()`.

- [ ] 3a: In `Execute()` (`subagent.go`), before the mode validation (line 364), add: if `in.AgentID != ""` and `in.Mode == ""`, set `in.Mode = "explore"`. This makes mode optional when resuming — the agent already exists, and "explore" is the cheapest safe default
- [ ] 3b: Update the tool definition schema (`subagent.go:104-127`): remove `"mode"` from the `required` array (only `"task"` is truly required). Update the `mode` field description to note it defaults to `"explore"` when `agent_id` is provided
- [ ] 3c: Add test: call `Execute()` with `{"agent_id": "...", "task": "summarize"}` and no `mode` field, verify it does not return a mode validation error (it may fail for other reasons like unknown agent_id — that's fine, the test just verifies mode defaults correctly)

## Phase 4: Tests for all three fixes together

- [ ] 4a: Add an integration-style test that exercises the combined scenario: create a mock sub-agent display with agents that complete at different times, verify (1) elapsed times are frozen and different per agent, (2) sub-agent lines appear before streaming text in rendered output
