---
id: RR-FQ302
type: review-response
title: KanbanPage.getColumn declared async but returns Locator
finding: getColumn is `async` but its annotation is `Locator` (should be `Promise<Locator>`). TS should flag this but doesn't because the tsconfig excludes pages/. Callers await it; `await` on a non-Promise is a no-op so it happens to work.
severity: critical
resolution: getColumn declared sync (returns Locator). Callers updated to drop await. Verified by typecheck.
status: addressed
---
