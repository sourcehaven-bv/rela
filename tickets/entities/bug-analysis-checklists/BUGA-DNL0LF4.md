---
id: BUGA-DNL0LF4
type: bug-analysis-checklist
title: 'Analysis: Portal polish: label sources; tab title; currency digits; git status request'
started: "2026-11-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (harbourmaster copy on rela-server from the branch)
- [x] Minimal reproduction steps documented (in the bug description)
- [x] Environment/conditions noted (fs backend without git sync, customer and staff users)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (schema label fallback; minor-unit digits; title hold; git capability)
- [x] Regression test planned (Go label tests, format, title and git store tests)
- [x] Related areas checked for similar issues (export headers and nested columns get the same fallback; the second invoices fetch is the live-view baseline refetch, by design; the 404s did not reproduce)
