---
id: RR-2G4IYP0
type: review-response
title: "BUG-6C3V prevention names the deleted usePageData"
finding: "The bug's prevention rule points new async loads at usePageData.ts, which knip removed."
severity: minor
resolution: "usePageData.ts is kept, because BUG-6C3V is still open and plans to adopt it; the bug text is unchanged."
status: addressed
---

The bug's prevention rule points new async loads at usePageData.ts, which knip removed.
