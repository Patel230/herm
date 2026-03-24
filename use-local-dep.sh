#!/usr/bin/env bash
#
# Set up (or tear down) a Go workspace so herm builds against a local
# clone of langdag instead of the module cache.
#
#   ./use-local-dep.sh              # clone langdag & enable workspace
#   ./use-local-dep.sh --reset      # remove workspace, track module version again
#
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
WORKSPACE="$ROOT/external-deps-workspace"
LANGDAG_DIR="$WORKSPACE/langdag"
LANGDAG_REPO="https://github.com/aduermael/langdag.git"
GITIGNORE="$ROOT/.gitignore"

# ── helpers ──────────────────────────────────────────────────────────

ensure_gitignore_entry() {
    local entry="$1"
    if ! grep -qxF "$entry" "$GITIGNORE" 2>/dev/null; then
        echo "$entry" >> "$GITIGNORE"
        echo "  added $entry to .gitignore"
    fi
}

# ── reset mode ───────────────────────────────────────────────────────

if [[ "${1:-}" == "--reset" ]]; then
    rm -f "$ROOT/go.work" "$ROOT/go.work.sum"
    echo "Removed go.work — herm now tracks the langdag module version in go.mod."
    echo "(external-deps-workspace/ was left in place.)"
    exit 0
fi

# ── setup mode ───────────────────────────────────────────────────────

# 1. Create workspace directory
mkdir -p "$WORKSPACE"

# 2. Ensure gitignore entries
ensure_gitignore_entry "external-deps-workspace/"
ensure_gitignore_entry "go.work"
ensure_gitignore_entry "go.work.sum"

# 3. Clone langdag if not already present
if [[ -d "$LANGDAG_DIR/.git" ]]; then
    echo "langdag already cloned at $LANGDAG_DIR"
else
    echo "Cloning langdag..."
    git clone "$LANGDAG_REPO" "$LANGDAG_DIR"
fi

# 4. Create go.work
cat > "$ROOT/go.work" <<EOF
go 1.24.0

use (
	.
	./external-deps-workspace/langdag
)
EOF

echo ""
echo "Go workspace enabled — herm will build against ./external-deps-workspace/langdag"
echo "Edit langdag locally, then 'go build' as usual."
echo ""
echo "To go back to the released version:  ./use-local-dep.sh --reset"
