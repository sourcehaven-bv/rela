---
id: RR-QWPJQFT
type: review-response
title: Whitespace-only lines treated as blank / break before a tab
finding: TrimSpace treated tab or long space lines as blank. A break could land before a tab.
severity: minor
resolution: isBlankIn accepts only spaces up to the content indent. breakPoint refuses a space next to a space or a tab. Test case for the tab.
status: addressed
---
