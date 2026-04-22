---
id: RR-XZ8D8
type: review-response
title: Inconsistent page-object navigation naming
finding: navigate() / navigateToEntity / navigateToCreateForm / navigateToKanban / navigateToList — pick one pattern. Either all generic `navigate(args)` or all `navigateToX()`.
severity: significant
reason: Naming is inconsistent but changing it touches every spec that instantiates a page object (~15 files). Defer to a focused rename ticket; the current names are at least descriptive, not cryptic.
status: deferred
---
