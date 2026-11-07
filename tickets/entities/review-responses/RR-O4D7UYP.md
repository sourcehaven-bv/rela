---
id: RR-O4D7UYP
type: review-response
title: Membership without org_id is now a 400
finding: A membership event without org_id used to reach the action and is now rejected; a custom single-tenant action would break.
severity: minor
resolution: Docs state the change; the reference action already failed without org_id.
status: addressed
---
