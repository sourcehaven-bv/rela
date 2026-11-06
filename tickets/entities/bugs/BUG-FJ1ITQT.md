---
id: BUG-FJ1ITQT
type: bug
title: File-store conflict-block refusal answers HTTP clients with a generic error
description: The file store refuses a body holding a complete git conflict block with an untyped error so HTTP writes answer 500 or a generic 422 instead of a typed 422.
priority: medium
effort: s
why1: fsstore returned an untyped errors.New for a document holding a conflict block; no caller could match it so dataentry fell through to its default mapping (500 on relation apply and 422 validation_failed elsewhere).
why2: BUG-GYM7ZBS added the refusal as a backstop the fix described as unreachable; the fenced-code case reaches it and its HTTP shape was never tested.
why3: The store package had no sentinel for content it refuses so every backend-specific refusal became an opaque error.
why4: The write-validation policy (structural impossibility is a 422) lives in prose in dataentry and nothing checks that a new store refusal has a typed error and an HTTP mapping.
why5: Store refusals are added per backend without a shared catalogue of sentinels that the HTTP layer maps in one place.
prevention: store.ErrConflictBlock is the typed refusal; writeInternalProblem answers it as 422 conflict_block so no path can 500 on it; TestWrite_ConflictBlockBodyIs422 pins create and PATCH and relation content over a real file store.
started: "2026-11-06"
completed: "2026-11-06"
status: done
---

## Description

Since PR #2101 the file store refuses to write an entity or relation body that
holds a complete git conflict block (for example one inside a fenced code
block), because it would read the file back as an unresolved merge. The refusal
is an untyped `errors.New` in `internal/store/fsstore`. Callers cannot match it,
so each HTTP write path maps it by default: a create answers 500, and other
paths answer 422 `validation_failed` or 500 depending on the handler.

The write-validation policy in `internal/dataentry/CLAUDE.md` classes a state
the storage layer cannot persist as a hard 422. The refusal must be a typed
store error that dataentry maps to a 422 with its own code, and MCP and CLI must
show a clear message. The message must not echo the body.

Follow-up to PR #2101.
