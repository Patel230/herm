# Remove Alternate Screen Buffer

**Goal:** Stop using the alternate screen buffer (`\033[?1049h`) so that native terminal scrolling works in all terminals (Zed, Ghostty, iTerm, Terminal.app, VS Code, etc.), and conversation history remains accessible in the terminal's scrollback.

**Why alt screen was used:** It provides an isolated buffer where absolute positioning (`\033[row;1H`) works predictably, exit is clean (terminal restores previous content), and resize is simple (clear + redraw).

**Why it must go:** Alt screen has no scrollback by design. Terminals like Zed and Ghostty strictly follow this — no scrollback means no scroll. iTerm/Terminal.app have proprietary workarounds, but depending on those isn't portable.

**Previous attempt failed because:** `renderFull()` used `\033[3J` (clear scrollback), which in normal-screen mode destroys the user's shell history AND can shift the viewport unpredictably, causing artifacts. The fix is to use `\033[H\033[2J` (home + clear visible screen) instead.

**Key insight from research:** herm's rendering already works without alt screen — `writeRows()` uses absolute positioning within the viewport (`\033[row;1H`), which targets the visible screen regardless of scrollback. The `scrollShift` overflow logic (write only visible rows at position 1) is viewport-relative and works in both modes. The only changes needed are: remove alt screen enter/exit, replace `\033[3J` with `\033[H\033[2J`, and fix shell-mode transitions.

---

## Phase 1: Remove alt screen and fix terminal setup/cleanup
- [ ] 1a: Remove `\033[?1049h` from startup and `\033[?1049l` from cleanup defer; keep bracketed paste and modifyOtherKeys (both work without alt screen); on exit, position cursor below the last rendered row and print a newline so the shell prompt appears in the right place
- [ ] 1b: Replace `\033[3J` (clear scrollback) with `\033[H\033[2J` (home + clear visible screen) in both `renderFull()` and the `render()` content-shrank path — this is the critical fix that prevents artifacts on resize
- [ ] 1c: Update shell-mode transitions (entering/exiting shell) — currently exit alt screen before shell and re-enter after; without alt screen, clear visible screen before shell, and clear + full re-render after shell returns

## Phase 2: Verify and fix edge cases
- [ ] 2a: Ensure the SIGWINCH → `renderFull()` path produces a clean re-render: hide cursor before clearing (`\033[?25l`), home + clear screen, write rows, show cursor — this eliminates flicker during resize
- [ ] 2b: Verify that the overflow render path (content > terminal height) works correctly: visible rows written at position 1, `\033[J` clears below, cursor positioned correctly via `scrollShift`
- [ ] 2c: Audit all remaining `\033[3J` usage — none should remain; `\033[3J` destroys scrollback and must not be used in normal-screen mode

## Phase 3: Add tests and verify cross-terminal behavior
- [ ] 3a: Add a test that verifies `render()` output contains `\033[H\033[2J` (not `\033[3J`) when doing a full re-render, and that `writeRows` output uses `\033[2K` per line and ends with `\033[J`
- [ ] 3b: Manual verification checklist: test in iTerm, Terminal.app, Ghostty, Zed terminal, VS Code terminal — verify: no artifacts on resize, native scroll works, clean exit leaves shell prompt in correct position, shell mode (/shell) works cleanly

---

**Success criteria:**
- Mouse/trackpad scroll works natively in all terminals (Zed, Ghostty, iTerm, etc.)
- Terminal resize produces no visual artifacts — content re-wraps cleanly
- Conversation history is accessible via native terminal scrollback
- Exiting herm leaves the terminal in a clean state with shell prompt below the conversation
- The robust CSI parser (already implemented) silently consumes any unknown escape sequences

**Open questions:**
- Should herm print a visible separator (e.g., `[HERM session]`) at the start of a session so the user can visually distinguish herm output from shell history when scrolling back?
- When entering shell mode, should we clear the screen or just position the cursor below? Clearing is cleaner but loses the visual context.
