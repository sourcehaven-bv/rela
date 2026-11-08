---
id: RR-4PZGCO4
type: review-response
title: Identity bind failure is only a warning
finding: BindOrWarn continues; the first local read fails with ErrNoIdentity. Fail early with an actionable message.
severity: minor
resolution: The local fingerprint runs before any request is sent, so a missing identity fails before contacting the remote (TestReconcileClient_SameViewOnBothSides); the CLI adds a hint on acl.ErrNoIdentity.
status: addressed
---
