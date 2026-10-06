---
id: RR-HBOKY4
type: review-response
title: Production GraphQuery literals are not classified
finding: D3 makes FaceSelection required on GraphQuery but section 5.2 classifies only EntityQuery literals. The 13 production GraphQuery literals include ACL principal lookup and traversal where the choice is security relevant.
severity: significant
resolution: 'Amendment A5: sentinels need no selection; dataentry list/scope/gantt/next-action use the request world; acl/readquery stays a template; acl traversal and principal lookup keep InWorld(zero) explicitly until Stage 3 and are listed in PR 8''s access delta.'
status: addressed
---

## Finding

D3 makes FaceSelection required on GraphQuery but section 5.2 classifies only
EntityQuery literals. The 13 production GraphQuery literals include ACL
principal lookup and traversal where the choice is security relevant.

Design: `.ignored/stage2-design.md` section 11.
