---
id: RR-M5LOLE0
type: review-response
title: CalDAV dynamic collection race
finding: dynamicMembers returned no restriction when the driver vanished between lookup and read, so a dynamic name could serve the whole member type.
severity: minor
resolution: dynamicMembers returns notFoundHere when the driver id is set but not found.
status: addressed
---
