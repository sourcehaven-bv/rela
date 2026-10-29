---
id: RR-P57G9T
type: review-response
title: The docs do not cover removed messages inside a run
finding: 'GUIDE-data-entry.md:1412 defines a run but does not say that a removed message between two members joins the run, counts in the fold line and shows as a placeholder when expanded. It also does not say that a removed message between hidden members disappears. Both rules are visible to readers (the count in ''3 messages: Progress'' includes placeholders). Fix: add one sentence to the run paragraph in the guide, then run just docs.'
severity: minor
resolution: Fixed. The run paragraph in GUIDE-data-entry.md says a removed message between members joins the run, counts in the line and shows as removed when expanded, and that in a run without a collapsed message it stays its own row; just docs regenerated docs/data-entry.md.
status: addressed
---
