---
name: manage-plans
description: Review, archive, and clean up plans. Use to archive completed plans, list plan status, or tidy the plans directory.
---

# Manage Plans

## Directory Layout

```
plans/              # Active plans only
plans/archive/      # Completed plans
```

- A plan is **completed** when it has zero `- [ ]` tasks and at least one `- [x]` task.
- A plan is **active** if it has any `- [ ]` tasks remaining.
- Only active plans live directly under `plans/`.

## Actions

### Archive completed plans (default)

Scan `plans/*.md`, identify completed plans, and move them to `plans/archive/`.

### Status report

List all plans (active and archived) with their task counts:

```
ACTIVE  plan-name.md  (3 pending, 7 done)
DONE    archive/plan-name.md  (0 pending, 12 done)
```

### Clean up

If the user asks to clean up:
1. Archive completed plans
2. Report status of remaining active plans
3. Flag any plans with no checkmarks at all (may be drafts or stale)

## Workflow

1. Read all `plans/*.md` files
2. Count `- [ ]` and `- [x]` lines in each
3. Perform the requested action (archive, status, or clean up)
4. Report what was done

## Invocation

$ARGUMENTS
