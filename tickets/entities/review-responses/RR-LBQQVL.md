---
id: RR-LBQQVL
type: review-response
title: Comments that justify a shape by the plimsoll cap
finding: 'internal/store/fsstore/index.go (scanTypeDir: ''A package function because FSStore sits at its plimsoll method line'') and internal/appbuild/storeschema.go:20. Turning a method into a package function that takes *FSStore keeps the same coupling and only hides it from the linter, and the comment admits that. scanTypeDir is fine as a free function on its merits (it needs only rooted and entitiesKey), so take (rooted, entitiesKey) instead of *FSStore and drop the plimsoll excuse. On the 36 to 37 bump: a separate refresher returned by New would be a second handle on the same locks and index. That is hidden coupling, not decomposition. One method with a documented reason is the honest shape, so keep it.'
severity: nit
resolution: scanTypeDir takes (rooted, entitiesKey); the plimsoll-justification comments are gone; the 36 to 37 bump has a one-line reason.
status: addressed
---
