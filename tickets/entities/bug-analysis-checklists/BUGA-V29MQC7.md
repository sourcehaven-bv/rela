---
id: BUGA-V29MQC7
type: bug-analysis-checklist
title: 'Analysis: Mail card at phone width, settings language and info, empty sidebar group, double not-found title, create sheet cut off'
started: "2026-11-23"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (rela-server on a copy of the pool project; the digest golden file in a 375 px browser)
- [x] Minimal reproduction steps documented (open Settings as a member; open an unknown path; open the "Add" sheet on a list at 375 px and read the body's scrollLeft)
- [x] Environment/conditions noted (saas-dev pool; any phone width; any app with one language)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (fluid mail card with a media query; app.languages from config to the shell and the SPA; neutral text; App Info behind can_configure; an empty label for loaded empty groups; an optional empty-state title; no sideways overflow in the sheet)
- [x] Regression test planned (mailrender compat test, dataentryconfig and staticshell tests, language and LanguageSetting tests, sidebarNav test, EntityList.empty test, mobile-layout e2e test)
- [x] Related areas checked for similar issues (the other empty states keep their title; the shell gets the language list before the config loads)
