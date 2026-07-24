---
id: RR-37IY
type: review-response
title: TKT-VMD8 AC7 missing symmetric next-skip case
finding: 'Hidden-middle test pins `prev` skipping. Mirror case — hidden entity AFTER the current — isn''t tested. With visible TKT-001 + hidden TKT-002, asking _position for TKT-001 must return next=nil, not next=TKT-002. If the ACL filter is correctly applied at scopedSortedEntities, TKT-002 is naturally out of the slice — but a regression test pinning the symmetric edge protects against a future ''optimize _position by skipping the filter'' mistake. Add to AC7: ''with visible TKT-001 and hidden TKT-002, _position for TKT-001 returns next=nil.'''
severity: significant
reason: The _position feature itself was removed from PR 2 scope per explicit
  user direction during TKT-VMD8, so its acceptance criteria (including this
  symmetric next-skip case, AC7) cannot be tested until the feature ships. The
  case is carried in the finding text verbatim so the future _position ticket
  can lift it directly into its AC list; testing it now would test dead code.
status: deferred
---
