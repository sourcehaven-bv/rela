---
id: RR-TQBZ2U
type: review-response
title: Trust boundary is conventional (doc + test) where it could be structural; no test that non-JWT resolvers cannot set roles
finding: 'The plan''s only protection against a header-sourced role is a doc section and a prose negative test. ChainResolvers (dataentry/router.go:440-450) advances purely on p.User != "" and ignores every other field, so nothing structural stops a future resolver returning {User: x, AssertedRoles: [...]} from a header — that would be full privilege escalation. Two asks. (1) Make the header-cannot-set-roles test a first-class table test over EVERY resolver (env router.go:348, header router.go:321, default router.go:295) asserting empty AssertedRoles, not a prose bullet. (2) Consider making the boundary structural rather than conventional: an unexported field plus a principal.Verified(...) constructor means AssertedRoles cannot be set by a struct literal at all, so all 21 non-test construction sites are compiler-prevented from forging roles. The codebase already shows this instinct in lua''s freezeTable. Cost: Principal stops being literal-constructible and JSON unmarshalling needs a custom UnmarshalJSON. Fallback is a lint test in the spirit of dataentry''s existing grep guards.'
severity: significant
resolution: 'Resolved with the user: take the structural option. principal.Principal gains unexported orgID/orgSlug/roles fields plus a principal.Verified(sub, tool, orgID, orgSlug, roles) constructor and accessor methods. Composite literals cannot set them, so all 21 non-test construction sites are compiler-prevented from forging roles — the trust boundary is enforced by the type system rather than by reviewer memory. Mirrors the freezeTable instinct in internal/lua (read-only contract enforced, not conventional). Accepted costs: Principal stops being fully literal-constructible, and audit.Record needs a custom MarshalJSON/UnmarshalJSON pair since Principal is embedded in a published wire format (audit.go:85-94). The unexported fields also resolve RR-3TKDR9 favourably: a struct with unexported slice fields is still non-comparable, so mcp/server.go:166 still needs its guard replaced — but the replacement is now unambiguous (IsZero-style method on Principal). The first-class table test over every resolver (env/header/default asserting empty roles) is still added; it is cheap and guards the runtime behaviour the type system cannot express.'
status: addressed
---

## Finding

The plan protects the trust boundary with a doc section and a prose negative
test. That is weaker than the invariant deserves.

`ChainResolvers` (`dataentry/router.go:440-450`) advances purely on `p.User !=
""` and ignores every other field. Nothing structural prevents a future resolver
from returning `{User: "x", AssertedRoles: [...]}` sourced from a header — which
would be a **full privilege escalation**, since `internal/acl` trusts
`Principal` absolutely.

## Resolution

**1. Make the negative test first-class.** A table test over *every* resolver —
env (`router.go:348`), header (`router.go:321`), default (`router.go:295`) —
asserting empty `AssertedRoles`. This is arguably the single most important
regression test in the ticket; it should not be a prose bullet.

**2. Consider making the boundary structural.** An unexported field plus a
constructor in `principal`:

```go
func Verified(sub, tool, orgID, orgSlug string, roles []string) Principal
```

means `AssertedRoles` **cannot be set by a struct literal at all** — all 21
non-test construction sites become compiler-prevented from forging roles. The
codebase already shows this instinct: `lua`'s `freezeTable` makes a read-only
contract enforced rather than conventional, and says so in its comment.

Cost: `Principal` stops being plainly literal-constructible, and JSON
unmarshalling needs a custom `UnmarshalJSON` (nothing in-repo unmarshals
`audit.Record` today, but it is a published wire format at `audit.go:85-94`).
Worth paying for a field whose entire security property is "only ever populated
after signature verification."

Fallback if that is too heavy: a lint test in the spirit of `dataentry`'s
existing grep guards — no file outside the JWT resolver may mention
`AssertedRoles:` in a composite literal.
