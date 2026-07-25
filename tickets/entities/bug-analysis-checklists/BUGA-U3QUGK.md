---
id: BUGA-U3QUGK
type: bug-analysis-checklist
title: 'Bug analysis: DisplayTitle bypasses hidden-primary-property fallback'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally — `TestACLViews_RedactsHiddenPrimaryTitle` leaks `SECRET-TITLE` as a section title without the fix.
- [x] Minimal reproduction steps documented — hide the primary property via `visible:`, GET `_views`, observe the title in a section's `fields[].values`.
- [x] Environment/conditions noted — backend-agnostic; the leak is `DisplayTitle` over raw properties in `sections.go`.

## Root Cause

- [x] Immediate cause identified (why1) — four surfaces call `DisplayTitle` directly, bypassing the hidden-primary fallback.
- [x] Contributing factors found (why2-3) — the fallback lived only in the serializer's `stripHiddenProperties`; `DisplayTitle` has no ACL awareness.
- [x] Systemic cause explored (why4-5) — title-safety and property-safety are two invariants enforced by one function; a title leak is invisible to property-map assertions.

## Fix Planning

- [x] Fix approach determined — shared with BUG-9QL9XV: routing `executeView` through `viewReader`, whose `Redact` recomputes `_title` on redaction.
- [x] Regression test planned — `TestACLViews_RedactsHiddenPrimaryTitle`, verified to fail without the fix.
- [x] Related areas checked for similar issues — the other three surfaces (mentions/analyze/settings) remain in this bug's scope for follow-up; the `_views` title leak is fixed here.
