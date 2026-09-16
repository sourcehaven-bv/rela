---
id: AM-shortcut-guards-use-shared-input-check
type: automated-measure
title: Every global keyboard-shortcut handler guards with isInputFocused(), not an ad-hoc tagName list
description: |-
    Two parts.

    1. A behavioural test per global single-key shortcut handler: with focus inside a `contenteditable` host (the Milkdown/ProseMirror shape), the shortcut must not fire. Covers Sidebar's `/` and SearchView's `f`.

    2. A source scan over `frontend/src/` that fails on an inline `['INPUT','TEXTAREA']`-style tagName comparison inside a keydown handler, directing the author to `isInputFocused()`.

    Catches the class that produced BUG-DNP5E7: a shared guard exists and is correct, but a handler predating it keeps a hand-rolled partial copy. The tagName copy is not obviously wrong on reading, and it degrades silently — the shortcut simply fires where it should not, which no type or lint rule detects. Part 2 is what makes the fix permanent; part 1 alone would leave the next hand-rolled guard uncaught.
kind: test
location: frontend/src/utils/inputGuardConvention.test.ts (source scan) + frontend/src/utils/dom.test.ts + frontend/src/components/common/Sidebar.shortcut.test.ts + frontend/src/views/SearchView.shortcut.test.ts
status: active
---

## Why a scan and not only a test

`isInputFocused()` already handles `INPUT`, `TEXTAREA`, `SELECT`,
`isContentEditable` and CodeMirror. The defect is not a gap in the guard, it is
handlers that do not use it. A behavioural test pins the two handlers that exist
today; the scan is what stops a third from being written.

## Scope of the scan

Flag an inline tag-name membership test (`['INPUT','TEXTAREA'].includes(...)`,
`tagName === 'INPUT'`, and near variants) appearing in a function that also
reads `KeyboardEvent`/`e.key`. Suppression requires a written reason, in line
with existing guard-test practice.
