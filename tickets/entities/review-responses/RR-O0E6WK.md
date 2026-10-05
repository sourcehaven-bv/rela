---
id: RR-O0E6WK
type: review-response
title: pg HighestID range bound not spelled out
finding: Section 3 names the ~>=~ / ~<~ range but not how the upper bound is built or why collation does not matter.
severity: nit
resolution: 'Amendment A11: upper bound is the prefix with the trailing - replaced by . and the pattern operators compare bytewise.'
status: addressed
---

## Finding

Section 3 names the ~>=~ / ~<~ range but not how the upper bound is built or why
collation does not matter.

Design: `.ignored/stage2-design.md` section 11.
