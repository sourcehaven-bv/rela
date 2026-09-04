---
id: RR-3TV7F4
type: review-response
title: 'An allowlist worlds: ceiling silently revoked the default world'
finding: |-
    VERIFIED. With baseline `worlds: [published]`, permitsWorld("default") and permitsWorld("") both returned FALSE. So an operator scoping a client to the published world silently took away its ability to read the default world.

    That is the same misreading the Q1 overrule spent a paragraph on, pointed the other way: the default world IS the draft face under the design doc's layout, so a client scoped to `published` would find its ordinary reads vanishing for a reason nothing in its config states. `worlds: [published]` reads as 'this client may ALSO reach published', not 'published and nothing else, including what it could already read'.

    It was fail-closed, so not a leak — but it was undocumented, untested, and a trap.
severity: significant
resolution: |-
    permitsWorld now admits the DEFAULT world unless it is EXPLICITLY denied (`deny_worlds: [default]`). A ceiling narrows what it NAMES, and the default world is the one world a grant never names — it is the absence of an entry.

    The explicit-denial path still delegates to worlds.permits, so a scope grant can re-open a denied default world via the existing `except` semantics. Two tests pin both halves: TestCeilingWorlds_AllowlistDoesNotRevokeDefaultWorld and TestCeilingWorlds_ScopeReopensDeniedDefaultWorld.
status: addressed
---
