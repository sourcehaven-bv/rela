---
id: RR-PA5B23
type: review-response
title: Trigger test gaps
finding: No pg trigger test, no sqlite relation or face cases, NotNil instead of equality.
severity: minor
resolution: Added TestVersionTriggersClearContentHash (pg), TestRelationVersionTriggersClearContentHash and VersionOnAnotherFace (sqlite), equality assertions.
status: addressed
---
