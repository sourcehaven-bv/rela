---
id: TKT-SI02JV
type: ticket
title: Reword list header/footer 'description' fallback note in docs
kind: docs
priority: low
effort: xs
status: done
---

## Summary

Docs-only follow-up to TKT-H7E611 / #1091. During code review of #1091, the
"legacy alias" framing for the list `description` field was corrected in the
code comments (review-response RR-GW5I5R): `description` was a previously-unused
field now adopted as a fallback for `header`, not a previously-rendered legacy
behavior. That reword landed after #1091 merged, so the docs still carried the
old "legacy alias" / "older name" phrasing.

This aligns `docs-project/entities/guides/GUIDE-data-entry.md` (and its
generated `docs/data-entry.md`) with the corrected comments.

## Scope

Docs-only, no behavior change. Two phrasings updated: the List Fields table row
for `description`, and the header/footer notes bullet.

## Verification

`just docs` regenerates cleanly; `just docs-check` passes (source and generated
output in sync).
