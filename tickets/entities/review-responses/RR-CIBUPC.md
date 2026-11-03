---
id: RR-CIBUPC
type: review-response
title: filechannels relies on caller passing SafeFS
finding: Atomic rewrite comes from SafeFS.WriteFile; a plain OsFS truncates in place. Wrap in SafeFS inside newStore as filecomments does.
severity: minor
resolution: newStore wraps a bare OsFS in SafeFS.
status: addressed
---
