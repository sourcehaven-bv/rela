---
id: REV-ZWTYAR
type: review-checklist
title: 'Review: Fix mobile layouts in Atlas'
status: done
---

## Automated Checks

- [x] Rela mobile Playwright suite passes (8 passed, 1 skipped)
- [x] Atlas create and relation flows pass at a Pixel 7 viewport
- [x] Dashboard, drawer, create sheet, and editor checked at 390×844

## Code Review

- [x] Self-reviewed the mobile layout changes for unrelated edits
- [x] No review findings remain

## Acceptance Verification

- [x] Dashboard title clears the menu button and starts at the top of the screen
- [x] Navigation drawer scrolls as one surface and hides the menu button while open
- [x] Create form fills the mobile viewport, uses the close button, and the editor spans the sheet
- [x] Detail labels remain legible on narrow screens
