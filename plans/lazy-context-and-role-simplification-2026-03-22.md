# Lazy Tool Context & Role Simplification

**Goal:** Reduce initial system prompt size by ~60% through lazy-loading tool guidance and skill content, and simplify the main agent role by removing the "orchestrator" identity.

**Motivation:** The main agent currently loads ~6,200 tokens of system prompt on every invocation — 55% of which is tool guidance that may never be used. The "orchestrator" framing is overfit: most tasks are simple and the agent should just act directly. Sub-agents are a tool, not an identity.

## Key files

- `cmd/herm/prompts/role.md` — three-branch role template (sub-agent / orchestrator / expert)
- `cmd/herm/prompts/tools.md` — 148 lines of tool guidance, all rendered upfront
- `cmd/herm/prompts/skills.md` — renders full skill content inline
- `cmd/herm/systemprompt.go` — prompt builder with PromptData flags
- `cmd/herm/systemprompt_test.go` — includes sub-agent ratio test (<60%)
- `cmd/herm/agent.go` — NewAgent, runLoop, tool execution
- `cmd/herm/subagent.go` — buildSubAgentTools, Execute
- `.herm/skills/devenv.md` — 135-line skill always embedded in prompt

## Design decisions

- **Tool JSON schemas (API `tools` param) stay eager.** The LLM needs these to know what tools exist. Only the system-prompt guidance text is deferred.
- **Guidance injection mechanism:** When the agent first invokes a tool, prepend the tool's extended guidance as a system message in the conversation. Track seen tools with a `map[string]bool`.
- **Role collapse:** The main agent is always "an expert coding agent." When the agent tool is available, the agent tool section in `tools.md` provides delegation guidance — no separate "orchestrator" identity needed.
- **Sub-agent tool filtering by mode:** Explore-mode sub-agents get read-only tools only. This reduces their tool definition count AND makes the behavioral boundary explicit.

## Success criteria

- System prompt is ≤50% of current size when no tools have been used yet
- Sub-agent ratio test still passes (<60% of main prompt)
- All existing systemprompt_test.go tests pass (adapted for new structure)
- Agent correctly receives tool guidance on first use of each tool
- Explore-mode sub-agents cannot call edit_file, write_file, or devenv

---

## Phase 1: Simplify main agent role

Collapse role.md from three branches to two. Remove the "orchestrator" identity.

- [ ] 1a: Rewrite `role.md` — merge the `HasAgent` and non-`HasAgent` branches into a single main agent role ("expert coding agent"). Keep the sub-agent branch (`IsSubAgent`) unchanged. Remove all delegation heuristics from role.md (lines 24-46 currently) — these move to the agent tool section in Phase 2
- [ ] 1b: Update `systemprompt_test.go` — the test checking for "orchestrator" or the three-branch structure needs updating. The sub-agent ratio test may shift; adjust the threshold if needed

## Phase 2: Lazy-load tool guidance

Split tool guidance into brief inline summaries (always in system prompt) and extended guidance (injected on first use).

- [ ] 2a: Split `tools.md` into inline summaries and per-tool extended guidance. The inline section keeps 1-2 lines per tool (name + what it does). The extended guidance for each tool becomes a separate named template (e.g. `{{define "guidance_bash"}}`) in a new `prompts/toolguide.md` file. Tools needing extended guidance: bash (container rules, don't install via bash), git (host vs container, merge conflicts, commit messages), devenv (workflow, Dockerfile rules, proactive build), agent (delegation heuristics moved from role.md, modes, when to use/not use, reading results)
- [ ] 2b: Add guidance injection to the agent loop. In `agent.go`, add a `seenTools map[string]bool` field to Agent. After a tool call completes for the first time, if extended guidance exists for that tool, inject it as a system-role message before the next LLM call. Add a `buildToolGuidance(toolName string) string` function to `systemprompt.go` that renders the per-tool template
- [ ] 2c: Update tests — verify that buildToolGuidance returns non-empty content for tools with guidance, and empty for tools without. Verify the slimmed-down system prompt still contains inline summaries for all tools

## Phase 3: Defer skills

Move skill content out of the initial system prompt. Inject when the corresponding tool is first used.

- [ ] 3a: Change `skills.md` template to render only the skill name+description list (the summary line), not the full `### {name}` content sections. Add a `buildSkillContent(skillName string) string` function to systemprompt.go
- [ ] 3b: Inject skill content on first relevant tool use. The devenv skill should be injected when devenv tool is first called. Wire this into the same guidance injection mechanism from Phase 2 — extend `seenTools` to also check for matching skills

## Phase 4: Filter sub-agent tools by mode

Explore-mode sub-agents should only get read-only tools.

- [ ] 4a: Modify `buildSubAgentTools()` in `subagent.go` to accept a `mode` parameter. When mode is `"explore"`, filter the tool list to: glob, grep, read_file, outline, bash. Exclude: edit_file, write_file, devenv. When mode is `"implement"`, keep the full tool set
- [ ] 4b: Update the sub-agent system prompt builder to reflect the filtered tool set (this should happen automatically since buildSubAgentSystemPrompt already checks tool names). Add a test verifying explore-mode sub-agents don't have write tools in their definitions
