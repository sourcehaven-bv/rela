---
id: RR-P90GOX
type: review-response
title: EntityQuery literal guard with an empty allowlist is optimistic
finding: Some literals are completed by a helper before execution; an empty allowlist from day one would force contortions.
severity: minor
resolution: 'Amendment A12: a shrink-only allowlist with a reason per entry; empty is the goal.'
status: addressed
---

## Finding

Some literals are completed by a helper before execution; an empty allowlist
from day one would force contortions.

Design: `.ignored/stage2-design.md` section 11.
