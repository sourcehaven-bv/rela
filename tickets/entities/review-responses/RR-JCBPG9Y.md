---
id: RR-JCBPG9Y
type: review-response
title: New TreeValue import sits inside the @/configure import run
finding: In about a dozen Configure views the new import type { TreeValue } from @/api/configure line sits between @/configure imports instead of with the @/api imports.
severity: nit
reason: 'Cosmetic only: no lint rule orders imports and the files already mix orders. Reordering a dozen files adds diff noise without changing anything a reader relies on.'
status: wont-fix
---
