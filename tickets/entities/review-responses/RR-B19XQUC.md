---
id: RR-B19XQUC
type: review-response
title: SPA labels legacy and beyond-cap values poorly
finding: 'A legacy title deep link shows a synthetic option beside the matching ''Title (ID)'' option, and an id beyond the fetch cap shows as a raw id. Fix: map a legacy title to the id when exactly one candidate has that title.'
severity: minor
resolution: EntityTargetSelect replaces a legacy title with its id when exactly one option has that title; a shared title is left as is. Covered by EntityTargetSelect and FilterBar tests.
status: addressed
---
