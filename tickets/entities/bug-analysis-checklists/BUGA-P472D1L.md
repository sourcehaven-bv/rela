---
id: BUGA-P472D1L
type: bug-analysis-checklist
title: 'Analysis: Open step scrolls the app shell frame and hides the top bar'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (appShellFrame.test.ts fails on develop: the frame has overflow: hidden)
- [x] Minimal reproduction steps documented (open an entity page with ?run=<run> on a phone-sized screen; the run card scrolls into view and the frame moves with it)
- [x] Environment/conditions noted (any backend; seen on the harbourmaster portal; worst on mobile)

## Root Cause

- [x] Immediate cause identified
- [x] Contributing factors found
- [x] Systemic cause explored

## Fix Planning

- [x] Fix approach determined (the frame uses overflow: clip, so nothing can scroll it; the run card gets a scroll margin for the entity page's sticky top bar)
- [x] Regression test planned (appShellFrame.test.ts)
- [x] Related areas checked for similar issues (every other scrollIntoView call is protected by the same change; the next-action Open step is already a RouterLink rendered as a link)
