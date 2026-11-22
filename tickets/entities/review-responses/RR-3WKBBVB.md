---
id: RR-3WKBBVB
type: review-response
title: "The read-only flag never resets"
finding: "Once set, the flag hid Create until a full reload, even after an operator made the workspace writable again."
severity: significant
resolution: "Addressed: a write that succeeds clears the flag. The live-update subscriptions PUT is exempt, because a read-only workspace still accepts it. Pinned by workspaceReadOnly.test.ts."
status: addressed
---
