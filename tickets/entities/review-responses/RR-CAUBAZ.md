---
id: RR-CAUBAZ
type: review-response
title: authorizeCommand's type switch fails OPEN on any unrecognized acl.ACL, including a pointer ReadOnlyACL
finding: |-
    authorizeCommand (internal/dataentry/commands.go:81) switches on the concrete ACL type with `default: return true`. Fail-open is the wrong default arm for an authorization boundary: an ACL implementation the switch does not recognize grants every command.

    CONCRETE BYPASS: acl.ReadOnlyACL.AuthorizeWrite has a VALUE receiver (internal/acl/readonly.go:21), so BOTH acl.ReadOnlyACL and *acl.ReadOnlyACL satisfy acl.ACL (Go method-set rule: a pointer's method set includes value-receiver methods). `case acl.ReadOnlyACL:` matches only the value form. Wiring appbuild.WithACL(&acl.ReadOnlyACL{}) compiles cleanly, falls through to default, and returns true — every command executes under a server started with --read-only. The exact bug this ticket exists to close, reintroduced through a pointer.

    VERIFIED NOT LIVE TODAY: grep for `&acl.ReadOnlyACL|&acl.NopACL|\*acl.ReadOnlyACL|\*acl.NopACL` across the repo returns ZERO hits; cmd/rela-server/main.go:126 passes the value form. So this is latent, not an exploitable bug in the shipped build.

    BUT the tree already contains a fourth acl.ACL implementation with no arm: *acl.Request (internal/acl/request.go:84) implements AuthorizeWrite. It is not wired as App.acl today (it is per-request and lives in context), but it is a real implementation that would hit `default` and fail open. The complete implementer set is: acl.NopACL (value), acl.ReadOnlyACL (value), *acl.Declarative (pointer, has an arm), *acl.Request (pointer, NO arm).

    appbuild.WithACL accepts any acl.ACL from any caller, and the guard against this is a code comment at commands.go:70 ("A future ACL implementation MUST get an explicit arm above"). A comment is not an enforcement mechanism, and nothing in the compiler or a test prevents the regression.

    RECOMMENDED (inverts the failure direction, closed by construction):

      switch a := aclImpl.(type) {
      case acl.NopACL:            // the ONLY fail-open arm
          return true
      case acl.ReadOnlyACL, *acl.ReadOnlyACL:
          return false
      case *acl.Declarative:
          ...
      default:
          return false            // unknown implementation ⇒ deny
      }

    This also subsumes the typed-nil gap: `if aclImpl == nil` (commands.go:77) catches only an untyped nil, so a typed-nil of any implementation other than *acl.Declarative currently reaches default and returns true. With default deny, it denies.
severity: significant
resolution: |-
    FIXED. Inverted the type switch so it is closed by construction (internal/dataentry/commands.go).

    - `default:` now returns FALSE. An acl.ACL implementation with no explicit arm denies rather than granting shell execution.
    - `case acl.NopACL, *acl.NopACL:` is now the ONLY fail-open arm, and is explicit rather than implicit.
    - `case acl.ReadOnlyACL, *acl.ReadOnlyACL:` matches both forms, closing the pointer bypass. Because AuthorizeWrite has a value receiver, &acl.ReadOnlyACL{} satisfies acl.ACL; under the old value-only match it fell to default and granted, which was a --read-only bypass reachable by adding one `&`.
    - The typed-nil gap is subsumed: any typed-nil other than *acl.Declarative now lands in default and denies.

    The godoc was rewritten to state the invariant rather than request it: 'Adding a new acl.ACL implementation? It denies commands until you add an arm. That is deliberate: the failure mode of forgetting is a denied command, not an ungoverned shell.'

    Pinned by TestAuthorizeCommandUnknownACLDenies with 4 subtests: pointer ReadOnlyACL denies, pointer NopACL still fails open, an unrecognized implementation (*acl.Request — the real fourth implementer identified in review) denies, and typed-nil Declarative denies. All pass.
status: addressed
---

## Why the current shape is risky even though it works today

Three separate things have to stay in agreement for the boundary to hold, and
none is checked by a test or linter:

1. the `default: return true` arm,
2. the convention that `ReadOnlyACL`/`NopACL` are always passed by value,
3. a comment telling future authors to add an arm.

Inverting the default collapses all three into one compiler-checked property: a
new or wrapped ACL denies until someone deliberately opts it into fail-open.

## Suggested test to pin it

```go
func TestAuthorizeCommandUnknownACLDenies(t *testing.T) {
	// A pointer ReadOnlyACL satisfies acl.ACL via the value receiver but is
	// a distinct dynamic type — it must not fall through to fail-open.
	if authorizeCommand(context.Background(), &acl.ReadOnlyACL{},
		CommandConfig{Context: "global"}) {
		t.Error("pointer ReadOnlyACL must deny")
	}
}
```
