---
id: BUGA-TRH3QE2
type: bug-analysis-checklist
title: 'Analysis: On mobile the hamburger overlaps the next-action banner'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (the new mobile e2e test 'the hamburger keeps clear of the next-action card' fails on develop: the banner starts 8px inside the button)
- [x] Minimal reproduction steps documented (on a phone, open an entity page that has a banner next action: the menu button covers the banner's top edge)
- [x] Environment/conditions noted (viewport up to 768px wide (Pixel 7 in the e2e mobile project), page without a sticky topbar, banner next action configured)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (give the button a bottom margin of 16px: 8px takes back the sticky offset and 8px is air; the existing -44px margin on pages with a sticky topbar still wins)
- [x] Regression test planned (the new e2e test measures at least 8px between the button and the banner)
- [x] Related areas checked for similar issues (pages with a sticky topbar overlay the button on the topbar and keep their -44px margin; the other mobile layout tests still pass)
