---
id: RR-K6PT93R
type: review-response
title: Two webhooks for one subject could undo a revocation
finding: The document lock covered only the read-modify-write. A slow older fetch could store after a newer revocation and keep the role.
severity: significant
resolution: 'The receiver holds a per-subject lock (idproles.Store.Hold: pg advisory key or striped lock files) from before the action fetches until Set returns. Pinned by TestHold_SerializesASubject and the hold assertions in TestWebhook_StoresAssertedRoles.'
status: addressed
---
