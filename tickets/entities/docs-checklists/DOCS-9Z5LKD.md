---
id: DOCS-9Z5LKD
type: docs-checklist
title: 'Documentation: Insert and edit external links in the Milkdown editor'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported functions and types have doc comments
- [x] Non-obvious decisions explained with WHY, not just what
- [x] ~~Package-level docs~~ (N/A: frontend TypeScript, no Go package docs)

Every new module carries a header explaining why it exists rather than what it
does. The load-bearing ones:

- `linkUrl.ts` — why the preset's `sanitizeLinkHref` is NOT delegated to (it
returns the unnormalized string), and why the gate runs on input only and never
on load.
- `linkSelection.ts` — why `findLinkAt` exists at all: `ToggleLink` is
`toggleMark`, whose `removeWhenPresent` tests `.some()`, so a partially
overlapping selection removes the link.
- `linkPaste.ts` — each guard states the data loss it prevents, since returning
`true` from `handlePaste` swallows the clipboard.
- `linkPanelPosition.ts` — why `TooltipProvider` is not used (it anchors to the
selection and throttles).

Three comments were corrected during review because they described behaviour the
code did not have. That is worse than no comment: it tells a reviewer not to
check. See RR-EOIEPJ (caret/stored marks), RR-W4CXNN (hover), and the `extentOf`
mark-equality note.

## Project Documentation

- [x] `docs/data-entry.md` updated
- [x] `frontend/CLAUDE.md` updated
- [x] ~~`docs/metamodel.md`, `docs/cli-reference.md`, `README.md`~~ (N/A: no
metamodel, CLI or project-level surface changed)

`docs/data-entry.md` § "The Markdown Body Editor":

- The construct list at the top of the section named the supported blocks and
would have become wrong the day this shipped; links and dividers added.
- The toolbar list gained link, divider, undo and redo.
- A new "Links to the web" paragraph states the accepted schemes, because a
refused `tel:` or relative link is otherwise a silent surprise with no
documented explanation. It also records the deliberate non-goal: a link already
in a file is left exactly as written even if its scheme would now be refused.

`frontend/CLAUDE.md` records the two invariants a future change is most likely
to break, both with the reasoning that makes them non-obvious: validate on input
never on load, and never dispatch `ToggleLink` for the link button. It also
warns that `@milkdown/components/link-tooltip` registers a colliding
`ToggleLink` slice that `commandNamesExistInEditor` cannot detect.

## External Documentation

- [x] ~~API docs / changelog~~ (N/A: no API surface change; this is UI-only)

## Verification

- [x] Documentation matches implementation
- [x] Examples are accurate

Verified during review rather than assumed — and this caught two errors:

1. The docs said the panel appears **above** the link. It appears below, which
is what the code does and what commit `00cd8b07` explicitly fixed.
2. The docs and four source comments described a **hover** trigger that was
never implemented (RR-W4CXNN). All such claims are removed; nothing now
documents behaviour that does not exist.
