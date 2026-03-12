# Token-Efficient File Exploration

**Goal:** Reduce agent token usage by adopting the file exploration strategies that make Claude Code efficient — dedicated search tools, partial file reads, sub-agent context isolation, and tool result management.

**Success criteria:**
- Agent uses significantly fewer tokens for equivalent tasks (measure before/after on a standard exploration task)
- File exploration feels faster due to less round-tripping through bash
- Sub-agents don't bloat the parent's context with raw file contents

---

## Current State

The agent today has:
- **One tool for file access: Bash** — all exploration goes through `cat`, `rg`, `tree`, `find`, etc.
- **Bash output truncation** at 200 lines / 30KB (keeps tail)
- **System prompt guidance** to explore in layers (structure → search → read)
- **Sub-agents** that return full text output to parent context
- **No partial file reading** — `cat` dumps the whole file
- **No dedicated search tools** — every search is a bash command, meaning the model must format CLI args AND parse raw terminal output, both consuming tokens

### Why This Is Expensive

1. **Bash overhead per tool call**: Model must generate the full command string, then parse unstructured terminal output. Claude Code's dedicated tools (Glob, Grep, Read) have structured inputs/outputs — the model writes `{"pattern": "*.go", "path": "src/"}` instead of `find src/ -name '*.go' -type f | sort`.
2. **Full file reads**: No way to read lines 40-60 of a file. The model gets the entire file every time.
3. **No result-level token management**: Tool results live in context forever. Claude Code clears old tool results and compresses conversation when context grows.
4. **Sub-agent output isn't summarized**: Full text dumps back into parent context.

---

## Phase 1: Dedicated File Exploration Tools

Add native tools that replace bash for common file operations. These return structured, minimal output.

- [ ] 1a: **Glob tool** — pattern-based file finder. Input: `pattern` (glob), optional `path` (directory). Output: sorted list of matching file paths. Uses Go's `filepath.Glob` or `doublestar` library for `**` support. No file contents, just paths.
- [ ] 1b: **Grep tool** — content search. Input: `pattern` (regex), optional `path`, optional `glob` (file filter), optional `context` (lines before/after), optional `output_mode` (`files_with_matches` default, `content`, `count`). Wraps `rg` or Go regex. Returns only matching lines/files, not full files.
- [ ] 1c: **Read tool** — file reader with partial support. Input: `file_path`, optional `offset` (start line), optional `limit` (max lines). Default: first 2000 lines. Truncates lines longer than 2000 chars. Returns content with line numbers (like `cat -n`). This replaces `cat`, `head`, `tail` for reading.
- [ ] 1d: **Update system prompt** to instruct the agent to prefer these tools over bash for file operations. Mirror Claude Code's guidance: "Do NOT use Bash to run cat, head, tail, grep, find, rg when a dedicated tool exists."
- [ ] 1e: **Test all three tools** — verify they work in the Docker container context (paths resolve to `/workspace`), handle edge cases (missing files, binary files, no matches), and return concise output.

**Key design decisions:**
- Tools execute inside the Docker container (same as bash), so file paths are relative to `/workspace`
- Output should be as compact as possible — no decorative formatting
- Glob defaults to `.gitignore`-aware if possible (use `rg --files -g` under the hood?)
- Read tool should clearly indicate truncation when it occurs

---

## Phase 2: Tool Result Token Management

Manage how much context tool results consume over a long conversation.

- [ ] 2a: **Research langdag's conversation retrieval** — understand how `GetAncestors()` builds the message chain. Can we modify which nodes are included? Can we mark nodes as "clearable"?
- [ ] 2b: **Implement tool result clearing** — when context grows beyond a threshold, replace old tool result content with a short placeholder (e.g., `[file content cleared — re-read if needed]`). Priority: clear largest/oldest results first. This mirrors Anthropic's `clear_tool_uses` API feature.
- [ ] 2c: **Implement conversation compaction** — when approaching context limits, summarize the conversation history. Use a cheap model (Haiku) to generate the summary. Replace the full history with: system prompt + summary + recent N turns.
- [ ] 2d: **Add `/compact` command** — manual trigger for compaction with optional focus hint.
- [ ] 2e: **Test compaction** — verify the agent maintains coherent behavior after compaction (remembers what it was doing, doesn't re-read files it already processed).

**Open questions:**
- Does langdag support modifying nodes in the conversation tree after creation? Or do we need to create a new branch?
- What's the right threshold for auto-compaction? Claude Code uses ~95% of context window.
- Should compaction be model-based (LLM summarizes) or rule-based (drop old tool results, keep reasoning)?

---

## Phase 3: Sub-Agent Context Efficiency

Make sub-agents cheaper and prevent their output from bloating parent context.

- [ ] 3a: **Route exploration sub-agents to a cheaper model** — when the sub-agent's task is primarily file discovery/reading, use Haiku instead of the main model. Add a model parameter to the agent tool or detect exploration tasks.
- [ ] 3b: **Summarize sub-agent output before returning** — instead of dumping full text back, have the sub-agent produce a structured summary. Option A: instruct sub-agent via prompt to be concise. Option B: post-process with a cheap model call. Option C: truncate to N tokens.
- [ ] 3c: **Give sub-agents access to the new dedicated tools** — sub-agents should use Glob/Grep/Read instead of bash for file operations, same as the main agent.
- [ ] 3d: **Test sub-agent token usage** — measure tokens consumed by sub-agent results in parent context before and after changes.

---

## Phase 4: Measurement & Tuning

- [ ] 4a: **Build a token usage benchmark** — create a standard task (e.g., "find and fix the bug in X", "explain how Y works") and measure total tokens consumed before and after changes.
- [ ] 4b: **Add token tracking to tool results** — log how many tokens each tool result consumes. Surface this in the UI or logs so we can identify expensive operations.
- [ ] 4c: **Tune thresholds** — adjust Read's default line limit, Grep's output cap, compaction trigger point, sub-agent output limits based on real usage data.
- [ ] 4d: **Compare against Claude Code** — run the same task in both agents, compare total token usage.

---

## Out of Scope (Future Work)

- **Repository map** (Aider-style): tree-sitter parsing + PageRank to build a compact codebase summary. High impact but high complexity. Consider after Phase 1-3.
- **Code intelligence / LSP integration**: Go-to-definition, find-references. Would replace many grep+read cycles but requires LSP server setup.
- **Token-efficient tool use API header**: Anthropic's `token-efficient-tools-2025-02-19` beta. Easy to add but separate from the exploration changes.
- **Prompt caching optimization**: Ensure system prompt and tool definitions hit Anthropic's prompt cache. Likely already happening but worth verifying.
