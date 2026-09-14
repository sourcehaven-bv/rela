---
id: BUGA-43K4UR
type: bug-analysis-checklist
title: 'Analysis: Document view flashes empty state and scrolls to top on any unrelated entity write'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

Confirmed by code trace rather than a live session: the blanking assignment
(`DocumentView.vue:100`) runs before the `await`, and the template's three-way
`v-if` chain (`:180-193`) has no branch that renders content when `docContent`
is empty and `showBlockLoader` has not yet fired. The empty-state fall-through
is therefore structural, not timing-dependent on the server. Any entity write of
any type reaches the client (`watcher.go:229`), so the trigger is not specific
to the viewed document. Steps recorded in the bug body.

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

Recorded in why1-why5. The key finding is why3: the anti-flash gate (TKT-TFSNBY)
was added around the blanking line without deleting it, so the `showBlockLoader`
comment describes behaviour the code never implemented.

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

Approach (client-side only, per the scope note in the bug body):

1. Blank `docContent` only on a cold load, not on an event-driven re-render.
2. Skip the assignment entirely when the fetched HTML equals the current value,
so an unchanged re-render performs no DOM write and the Mermaid/PlantUML watcher
does not re-run.
3. On error, preserve existing content rather than clearing it - clearing
replaces a readable document with an empty state on a transient failure.

Regression test: AM-document-rerender-preserves-content. It must observe the
INTERMEDIATE render, since assertions on the settled state pass against the
buggy code.

Related areas: `DocumentsPanel.vue` carries the identical copy-pasted defect
(blanking at :124/:148, unconditional SSE re-render at :158-160, same template
chain and same misleading comment). Included in this fix. Other `.value = ''`
sites (`ConflictsView`, `DynamicForm`, `HelpModal`) are form/modal resets on
user action, not SSE-driven re-renders, and are not affected.
