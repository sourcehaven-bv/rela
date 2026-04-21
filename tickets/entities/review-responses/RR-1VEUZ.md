---
id: RR-1VEUZ
type: review-response
title: WalkAll reports root as '.' — magic sentinel the user has to know about
finding: '''.'' is rejected by resolve(), so if a caller sees it in a WalkAll callback and feeds it back into another RootedFS method, they get a confusing error. The casual doc mention wasn''t prominent enough.'
severity: nit
resolution: 'Upgraded the WalkAll doc comment to an explicit NOTE callout: root reports as ''.'', ''.'' is not a valid key, and explains how callers should navigate (ReadDir with a first-level subkey, not by feeding ''.'' back).'
status: addressed
---
