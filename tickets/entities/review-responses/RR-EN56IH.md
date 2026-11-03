---
id: RR-EN56IH
type: review-response
title: Family delete hidden-face claim depends on WriteTarget
finding: AnchorDelete.Block doc and docs/acl-security.md say a hidden face never changes the answer; that holds only because callers resolve through WriteTarget first.
severity: nit
resolution: The AnchorDelete.Block doc and docs/acl-security.md state that the property depends on callers resolving through WriteTarget first.
status: addressed
---
