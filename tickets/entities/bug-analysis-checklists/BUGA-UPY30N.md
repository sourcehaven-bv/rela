---
id: BUGA-UPY30N
type: bug-analysis-checklist
title: 'Bug analysis: ACL-hidden properties leak through _views section field values'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally — regression test `TestACLViews_RedactsHiddenPropertyValue` leaks `SECRET-STATUS` through `_views` `fields[].values` without the fix.
- [x] Minimal reproduction steps documented — hide a rendered property via `visible:`, GET `/api/v1/_views/{type}/{id}`, observe the value in a section.
- [x] Environment/conditions noted — any build; the leak is in `sections.go`, backend-agnostic.

## Root Cause

- [x] Immediate cause identified (why1) — `sections.go` reads raw `e.Properties` into `Values`.
- [x] Contributing factors found (why2-3) — the view path loaded raw from `a.store`, bypassing the redacting reader; strip was per-field convention.
- [x] Systemic cause explored (why4-5) — field visibility enforced per-surface, not at a wire-boundary invariant. The choke point (`visibility.PolicyReader`) existed; the view path predated it.

## Fix Planning

- [x] Fix approach determined — route `executeView` output through `viewReader` (`visibility.PolicyReader`); no per-field strip in `sections.go`.
- [x] Regression test planned — two `_views` field-redaction tests, verified to fail without the fix.
- [x] Related areas checked for similar issues — survey found the title variant (→ BUG-R9EHKV) and other surfaces (sync/mentions/analyze/settings) tracked separately.
