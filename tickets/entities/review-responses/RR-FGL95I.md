---
id: RR-FGL95I
type: review-response
title: just fuzz-all red-by-default until BUG-RHFHTH fixed
finding: FuzzGenerateShortID reliably fails, so the new recipe and the first weekly run are guaranteed red with no warning — erodes trust in the harness on day one.
severity: significant
resolution: Known-red state documented on the recipe (naming BUG-RHFHTH); the first scheduled run filing an issue for it is the system working as designed. Fixing the bug itself is the next PR (BUG-RHFHTH is ready with full repro).
status: addressed
---
