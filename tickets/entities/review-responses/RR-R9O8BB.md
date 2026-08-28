---
id: RR-R9O8BB
type: review-response
title: hidesNavEntry re-read the schema mid-request, violating capture-state-once
finding: 'hidesNavEntry called h.schema() per navigation entry while handleV1Sidebar had already captured s := h.schema() at the top. The root CLAUDE.md rule is explicit that multiple loads against the underlying atomic.Pointer can observe different snapshots. Concretely: a config reload landing mid-loop means the navigation list comes from the old snapshot and the documents map from the new one; a renamed document then misses the lookup, hits the !ok fail-open branch, and a gated entry renders.'
severity: significant
resolution: MOOT. The fix was correct for the code as it stood, but hidesNavEntry no longer exists — sidebar filtering was removed entirely (see RR-E8Z1MR), since menu structure is principal-independent by an already-recorded decision in docs/acl-security.md. The underlying capture-state-once rule still stands and is unchanged in the root CLAUDE.md; there is simply no longer a call site here to get it wrong.
reason: |-
    Moot rather than rejected: the code the finding described no longer exists. hidesNavEntry was deleted along with all sidebar filtering (see RR-E8Z1MR), because menu structure is principal-independent by an already-recorded decision in docs/acl-security.md.

    The finding itself was correct and its fix was applied at the time — passing the caller's captured *Schema instead of re-reading it mid-loop. It is only being closed as won't-fix because there is no longer a call site that could get it wrong. The underlying capture-state-once rule in the root CLAUDE.md is unchanged and still binding on all other code.
status: wont-fix
---
