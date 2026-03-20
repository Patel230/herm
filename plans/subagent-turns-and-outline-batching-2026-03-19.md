# Sub-agent Turn Counting Fix & Outline Batching

**Problem:** Sub-agents hit their turn limit too often because turns are counted per
tool call (`EventToolCallStart`) instead of per LLM response. The main agent counts
per iteration (1 LLM response + all its tool calls = 1 iteration), but the sub-agent
counts each individual tool call as a turn. If the LLM batches 5 `outline` calls in
one response, that consumes 5/15 of the budget for what the main agent considers 1
iteration.

**Three fixes:**
1. Count sub-agent turns per LLM response cycle, not per tool call
2. Bump default from 15 to 20
3. Add multi-file support to `outline` tool to reduce tool call volume

---

## Phase 1: Fix sub-agent turn counting

**Key files:** `cmd/herm/subagent.go`, `cmd/herm/subagent_test.go`

**Event flow context:** The agent emits events in this order per LLM response:
- `EventTextDelta` (streaming text, zero or more)
- `EventToolCallStart` (one per tool call in the batch)
- `EventToolCallDone` (one per tool, after execution)
- `EventUsage` (once per LLM response — the response boundary marker)
- Loop repeats for next LLM response, or `EventDone`

**Approach:** Use a `responseCounted` bool. On `EventToolCallStart`, if not already
counted for this response, increment `turns` and set `responseCounted = true`. On
`EventUsage` (which fires once per LLM response), reset `responseCounted = false`.
This way, 5 tool calls in one response = 1 turn. The limit check stays on
`EventToolCallStart` for fast cancellation.

Also bump `defaultSubAgentMaxTurns` from 15 to 20 and update comment from
"tool-call cap" to "response-cycle cap".

- [ ] 1a: Change turn counting in `Execute()` event loop (lines 195-206) — add `responseCounted` bool, increment on first `EventToolCallStart` per response, reset on `EventUsage`
- [ ] 1b: Update `defaultSubAgentMaxTurns` constant (line 17-18) from 15 to 20 and update comment
- [ ] 1c: Update `prompts/tools.md` line 135 — change wording to reflect response-based counting
- [ ] 1d: Update tests in `subagent_test.go` — fix `TestSubAgentResultMaxTurnsShown`, `TestFormatSubAgentResult`, and any hard-coded turn expectations; add a new test that verifies multiple tool calls in one response count as 1 turn (will need `scriptedProvider` from `agent_test.go` or equivalent to emit multiple `EventToolCallStart` events)

**Success criteria:**
- A sub-agent LLM response with N tool calls counts as 1 turn, not N
- Default limit is 20
- Existing tests pass with updated expectations
- New test confirms batched tool calls = 1 turn

---

## Phase 2: Multi-file outline tool

**Key files:** `cmd/herm/filetools.go`, `cmd/herm/filetools_test.go`, `cmd/herm/prompts/tools.md`

**Current state:** `OutlineTool` accepts `{"file_path": "..."}` — one file per call.
The input struct is `outlineInput{FilePath string}`.

**Approach:** Add `file_paths` (array of strings) to the input schema alongside
`file_path`. When `file_paths` is provided, iterate over each path, call the outline
binary for each, and return combined output with file headers. Keep `file_path`
backward-compatible — if only `file_path` is given, behave exactly as today.

Cap `file_paths` at a reasonable limit (e.g., 20 files) to prevent abuse.

- [ ] 2a: Extend `outlineInput` struct to add `FilePaths []string` field; update `InputSchema` in `Definition()` to include `file_paths` array property; update `Description` to mention multi-file support
- [ ] 2b: Refactor `Execute()` — extract single-file logic into a helper, add multi-file loop that calls the helper for each path and combines results with `=== <path> ===` headers between files
- [ ] 2c: Update `prompts/tools.md` outline description to mention multi-file support
- [ ] 2d: Add tests — `TestOutlineTool_Execute_MultipleFiles` (happy path), `TestOutlineTool_Execute_MultipleFiles_PartialError` (one file fails, others succeed), `TestOutlineTool_Execute_MultipleFiles_TooMany` (over cap returns error), `TestOutlineTool_Execute_BothInputs` (file_path + file_paths merges correctly)

**Success criteria:**
- `{"file_paths": ["a.go", "b.go"]}` returns combined outline with file headers
- `{"file_path": "a.go"}` still works unchanged
- Over-cap input returns a clear error
- Partial failures include error messages inline without aborting other files
