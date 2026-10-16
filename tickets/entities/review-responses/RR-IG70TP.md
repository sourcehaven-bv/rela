---
id: RR-IG70TP
type: review-response
title: Owned set not guarded when nothing detached
finding: guardDeletes returns a nil owned set when nothing was detached, so requireGuardedOwned no longer refuses an owned entity that appeared after the scope was taken; a block-mode channel on it is bypassed.
severity: significant
status: open
---
