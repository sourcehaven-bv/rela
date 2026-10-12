---
id: RR-GU8XVZ
type: review-response
title: ReplaceOutgoing removed hidden edges and decided delete ACL outside the transaction
finding: Automated security review of 93f44f514. ReplaceOutgoing removed every edge of the tail, including edges the caller cannot read. The delete ACL was decided outside the transaction, so the edge set could change between the check and the write (TOCTOU).
severity: significant
resolution: Fixed in 6f21af1dc and replaced by ReplaceRelations in 21e383051. A replace removes only an explicit list of named edges. The ACL is decided from that fixed list. Covered by TestReplaceRelations_LeavesUnnamedEdges and TestReplaceRelations_DeniedRemoveWritesNothing.
status: addressed
---

Automated security review finding on 93f44f514.
