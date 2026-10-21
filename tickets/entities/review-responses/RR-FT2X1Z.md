---
id: RR-FT2X1Z
type: review-response
title: Inline edit can close mid-request
finding: With one offer and no dialog, clicking outside ends the inline edit during the request; the item is never linked and a retry duplicates.
severity: minor
resolution: While a request runs, TaskItemConversions.panels() returns the whole document, so a press elsewhere does not end the inline edit. Escape still ends it as an explicit act, and the result is then reported as a toast. Pinned by a component test.
status: addressed
---
