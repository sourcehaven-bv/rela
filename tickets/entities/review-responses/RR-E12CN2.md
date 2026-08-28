---
id: RR-E12CN2
type: review-response
title: Docs claimed the reject 403 'leaks nothing'; the CRUD path returns the reason string
finding: 'GUIDE-acl-security claimed ''The 403 leaks nothing.'' Verified false on the CRUD/attachment/conflict-resolve paths: writeForbiddenIfACLDenied (settings_handlers.go:76-88) encodes {error, rule_kind, rule_id, reason}, so a rejected PATCH returns reason=''verified principal resolves to no user entity; writes are rejected''. The sync path (sync_handlers.go:354) correctly returns a generic ''Not permitted'' with no reason — so the two write surfaces disagree, and the docs overstate non-disclosure. Not a security hole: the caller already knows it''s unmatched, and every ACL denial on this path exposes reason/rule_kind by design for the SPA affordance contract. But the docs contradict the code.'
severity: minor
resolution: 'Fixed the docs, not the code — suppressing the reason would break the uniform affordance-denial shape every other ACL deny uses. GUIDE-acl-security now states the 403 carries rule_kind/reason like every ACL denial, discloses only that the identity is unmatched (which the caller already knows), and reveals no policy detail beyond the rule that fired. The reviewer confirmed the reason string is not sensitive. Also took two reviewer nits: a comment pinning ''SetJWTGate must precede NewRouter'' (the jwtVerified snapshot invariant, router.go), and expanded the anti-bypass test comment to name attachment/rename/clone/relation/conflict-resolve as covered via Declarative.AuthorizeWrite (conflict-resolve calls it directly, not via entitymanager), pinned at unit level by TestUnmatchedPrincipal_RejectDeniesFlaggedWrite.'
status: addressed
---

## Finding

`GUIDE-acl-security` claimed *"The 403 leaks nothing."* False on the
CRUD/attachment/conflict-resolve paths: `writeForbiddenIfACLDenied`
(`settings_handlers.go:76-88`) encodes `{error, rule_kind, rule_id, reason}`, so
a rejected `PATCH` returns `reason: "verified principal resolves to no user
entity; writes are rejected"`. The **sync** path (`sync_handlers.go:354`)
returns a generic `"Not permitted"` — the two write surfaces disagree, and the
docs overstate non-disclosure.

Not a security hole (the caller already knows it's unmatched; every ACL denial
on this path exposes `reason`/`rule_kind` by design for the affordance
contract), but a real doc-vs-code contradiction.

## Resolution

Fixed the docs, not the code — suppressing the reason would break the uniform
affordance-denial shape. The guide now states the 403 carries `rule_kind`/
`reason` like every ACL denial, discloses only "unmatched" (which the caller
already knows), and no policy detail beyond the rule that fired.

Plus two reviewer nits: a comment pinning "SetJWTGate must precede NewRouter"
(the `jwtVerified` snapshot invariant), and an expanded anti-bypass test comment
naming attachment/rename/clone/relation/conflict-resolve as covered via
`Declarative.AuthorizeWrite` — conflict-resolve calls it directly rather than
through entitymanager, pinned at unit level by
`TestUnmatchedPrincipal_RejectDeniesFlaggedWrite`.
