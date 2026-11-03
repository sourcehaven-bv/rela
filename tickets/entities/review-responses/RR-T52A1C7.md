---
id: RR-T52A1C7
type: review-response
title: No test at the search level
finding: The new tests call filter.Match only. Nothing runs prop:x=~y through _search with an enum property.
severity: minor
reason: The data-entry test schema has no enum on the search path. The end-to-end search was verified manually against the demo project. BUG-4WKAMFF will make such a regression visible as an error.
status: deferred
---
