---
id: BUG-J6QZUZ
type: bug
title: apps.spec.ts toolbar-active assertions are flaky
description: 'Two apps.spec.ts tests for the sandboxed app editor toolbar fail intermittently — reproduced 3 times in 20 runs on unmodified develop (bbce47a0). Both read toolbar active-state immediately after a click, racing ProseMirror update cycle instead of waiting for it. Surfaced on PR #1664, whose diff touches neither the spec nor src/app-editor/.'
priority: medium
effort: s
status: backlog
---

## Description

Two `apps.spec.ts` tests for the sandboxed app editor's toolbar fail
intermittently. Reproduced on unmodified `develop` (bbce47a0): **3 failures in
20 runs** of just these two tests at `--repeat-each=10 --workers=5`.

- `apps.spec.ts:182` "formats through the toolbar and reports the result on
.value" — `expect(await app.editorCommandActive('h1')).toBe(true)` gets false
- `apps.spec.ts:201` "disables a command that would do nothing where the cursor
is"

Surfaced on PR #1664, whose diff touches neither the spec nor `src/app-editor/`.
That run was 1 failed / 328 passed.

## Likely cause

Both assertions read toolbar state immediately after `clickEditorLine(...)`.
ProseMirror updates its chrome on its own update cycle, so the active-state
probe can run against the state before the selection lands — the same
one-tick-behind hazard `frontend/CLAUDE.md` already documents for the mention
menu's `query` mirror ("written by the slash provider's `shouldShow`, which runs
on ProseMirror's update cycle — after the capture-phase key handler").

A bare `expect(...)` does not retry. `expect.poll` (or a web-first assertion on
the button's pressed state) would wait for the cycle instead of racing it.

## Why it matters

It fails unrelated PRs, which trains reviewers to re-run red CI without reading
it — the habit that lets a real failure through.
