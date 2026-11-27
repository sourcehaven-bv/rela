---
id: RR-2G4IYP0
type: review-response
title: "BUG-6C3V prevention names the deleted usePageData"
finding: "The bug's prevention rule points new async loads at usePageData.ts, which knip removed."
severity: minor
resolution: "BUG-6C3V now points at isCancelledFetch in api/errors.ts and records the removal."
status: addressed
---

The bug's prevention rule points new async loads at usePageData.ts, which knip removed.
