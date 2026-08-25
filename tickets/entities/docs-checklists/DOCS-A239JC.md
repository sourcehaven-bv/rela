---
id: DOCS-A239JC
type: docs-checklist
title: 'Docs: Review checklist must not track PR URL or CI status'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] ~~Exported symbols documented~~ (N/A: no code)
- [x] Non-obvious decisions carry a WHY — the template's omission is the whole
point of the change, so it carries an inline comment explaining what is
deliberately absent and why, citing TKT-UFV01M
- [x] `CLAUDE.md` updated — workflow steps reordered to match enforcement,
with the reason the PR URL is not recorded

## Project Documentation

- [x] `CLAUDE.md` — "Agent Workflow for Tickets" steps 4 and 5
- [x] `tickets/templates/entities/review-checklist.md` — the template itself
- [x] ~~`.claude/commands/pr.md`~~ (N/A: verified it never instructs writing
the URL into a checklist. Its two PR-URL references are for monitoring the run
and for the command's own final report, both of which stay.)
- [x] ~~`docs/` guides~~ (N/A: contributor workflow, not a user-facing rela
feature. `docs/` is generated from the docs-project graph and describes rela to
its users; the ticket workflow governs this repo's own process.)
- [x] ~~`README.md`~~ (N/A: same reasoning)

## External Documentation

- [x] ~~Tool README~~ (N/A: no tool involved)
- [x] ~~Migration notes~~ (N/A: the 171 existing checklists are deliberately
left as-is and keep validating; nothing to migrate)

## Verification

- [x] Every documented command was run and produces the documented output —
`rela validate --project tickets` passes; `just lint-md` passes
- [x] The documented workflow order matches the enforced one — this was the
second defect found, and `CLAUDE.md` now reads Complete → Create PR, matching
`/pr`'s done-before-PR gate
