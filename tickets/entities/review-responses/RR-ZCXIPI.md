---
id: RR-ZCXIPI
type: review-response
title: widenWorlds hard-codes the default world name
finding: Compared against the literal 'default' although WorldInfo.default exists for that purpose.
severity: minor
resolution: Skips on info.default and on DEFAULT_WORLD (for older servers); ambientWorld uses DEFAULT_WORLD. Unit test added.
status: addressed
---
