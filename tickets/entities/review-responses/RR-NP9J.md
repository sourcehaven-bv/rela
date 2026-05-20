---
id: RR-NP9J
type: review-response
title: Error message + SPA banner hardcode git-crypt, locking out future SOPS/ACL extensions
finding: 'api_v1.go:529 detail message says ''File is git-crypt encrypted; run `git-crypt unlock` first.'' EntityDetail.vue:484-495 banner names git-crypt explicitly. But InaccessibleReason is an enum designed (per its doc comment) to extend to SOPS, Lua-driven ACLs, etc. When SOPS lands, every consumer that produces this string changes. Fix: derive the detail message from entity.Inaccessible[0].Reason (already on the wire); ship a per-reason remediation map OR have the SPA branch on reason to render the right banner. Forward-compat for the enum the type signature already commits to.'
severity: significant
status: deferred
reason: |-
    Parent ticket TKT-PGK91 (git-crypt detection) shipped via PR #668 without addressing this finding. Captured here so the gap remains visible; will be revisited if the underlying code path becomes a problem in practice. Closed as deferred via the TKT-5S8T data-debt sweep — the alternative is leaving the RR open indefinitely while it blocks every unrelated PR.
---
