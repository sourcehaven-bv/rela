---
id: RR-NKTI9X
type: review-response
title: TestPrincipal_AccessorsDoNotShareBackingArray was a tautology, and a commit message vouched for it
finding: 'The test added for RR-S14ZKC asserted `&a.Roles()[0] == &b.Roles()[0]`. Roles() clones unconditionally, so both sides are freshly allocated and the comparison can never be true — the test passed with Clone() fully reverted. Verified by simulating pre-fix From(): the unexported roles field genuinely aliased (true), while the assertion the test actually made read false. It was an external (package principal_test) test asserting on an invariant only observable on an unexported field. Worse than the missing coverage: commit b1657597''s message claimed ''Both regression tests fault-injected to confirm they fail without the fixes.'' That was true of the two acl tests and FALSE of this one — I did not fault-inject it and should not have vouched for it. A tautology in a security test file is worse than no test, because it advertises coverage that does not exist.'
severity: significant
resolution: 'Deleted and replaced with in-package tests (internal/principal/clone_internal_test.go, package principal) that assert on the unexported roles field directly: TestClone_BreaksBackingArraySharing and TestFrom_DoesNotAliasTheContextValue. Both fault-injected — with Clone made a no-op and removed from From(ctx) they fail with ''Clone shares the roles backing array with the original'' and ''two From(ctx) results share the roles backing array''. Added TestClone_ZeroRolesDoesNotAllocate pinning that the no-roles path (every CLI/MCP/scheduler/header request) stays allocation-free, so only the JWT path pays for the guarantee. The file header explains why these must be in-package, naming the tautology so it is not reintroduced.'
status: addressed
---

## Finding

`TestPrincipal_AccessorsDoNotShareBackingArray` asserted `&a.Roles()[0] ==
&b.Roles()[0]`. `Roles()` clones unconditionally, so both sides are freshly
allocated and the comparison **can never be true** — the test passed with
`Clone()` fully reverted.

Verified by simulating the pre-fix `From()`:

```text
unexported roles alias (real sharing):                    true
assertion my test made (&a.Roles()[0]==&b.Roles()[0]):    false
```

The bug is genuinely present, and the assertion still reads `false`. It was an
external (`package principal_test`) test asserting on an invariant only
observable on an unexported field.

**Worse than the missing coverage:** commit b1657597's message claimed *"Both
regression tests fault-injected to confirm they fail without the fixes."* That
was true of the two `acl` tests and **false of this one** — I did not
fault-inject it and should not have vouched for it. A tautology in a security
test file is worse than no test, because it advertises coverage that does not
exist.

## Resolution

Deleted; replaced with in-package tests (`clone_internal_test.go`, `package
principal`) that assert on the unexported `roles` field directly:

- `TestClone_BreaksBackingArraySharing`
- `TestFrom_DoesNotAliasTheContextValue`

Both fault-injected — with `Clone` made a no-op and removed from `From(ctx)`
they fail with *"Clone shares the roles backing array with the original"* and
*"two From(ctx) results share the roles backing array"*.

Also added `TestClone_ZeroRolesDoesNotAllocate`, pinning that the no-roles path
— every CLI, MCP, scheduler and header-auth request — stays allocation-free, so
only the JWT path pays for the guarantee. (Measured: 0 allocs without roles, 1
with.)

The file header explains why these must be in-package, naming the tautology so
it is not reintroduced.
