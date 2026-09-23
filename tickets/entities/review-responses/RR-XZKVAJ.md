---
id: RR-XZKVAJ
type: review-response
title: loadSidebar applies out-of-order responses
finding: Mount and each refresh start independent getSidebar calls and every response was applied so an older response could overwrite newer navigation.
severity: minor
resolution: A request counter drops any response overtaken by a later load. Test added and mutation-checked.
status: addressed
---
