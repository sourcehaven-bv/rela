---
id: RR-ZKCLN3
type: review-response
title: Symlinked type folder gets the wrong reason
finding: A symlinked folder under entities/ was listed as an undeclared type.
severity: nit
resolution: checkSymlinks now refuses any symlink under entities/ before reconciliation runs.
status: addressed
---
