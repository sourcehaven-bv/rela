---
id: RR-3M94VD
type: review-response
title: Denial test should assert which face was denied
finding: The denial test only checked that some ForbiddenError came back.
severity: nit
resolution: Added a recording ACL wrapper (deniedFaceACL). Tests assert drafter is denied published and publisher is denied draft.
status: addressed
---
