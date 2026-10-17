---
id: RR-5TJ2OD
type: review-response
title: Reload does not consult stored type records
finding: CheckReload compares only the previous and next config. Removing a type, reloading, then re-adding it with another anchor is accepted; stored channels are read under the wrong keys and the next boot refuses.
severity: significant
status: open
---
