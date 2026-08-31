---
id: RR-I1RTSU
type: review-response
title: 'Adding a Policy() call to NewRouter widened the blast radius of a typed-nil ACL'
finding: |-
    A typed-nil `*acl.Declarative` in the `a.acl` interface is not nil, so the
    `d != nil` guard in NewRouter's type switch is load-bearing. Adding
    `d.Policy()` there made getting it wrong worse: previously a typed-nil failed
    later, per request; now NewRouter itself panics at construction.

    Removing the guard left every test in the file green -- confirmed, and confirmed
    that it then panics with a nil dereference.
severity: significant
resolution: |-
    Two changes, defense in depth:

    - `Declarative.Policy()` is now nil-receiver safe, returning a nil *Policy. The
      Policy predicates each handle a nil receiver, so a typed-nil now gives the
      same "no policy" answer as an absent one instead of panicking.
    - `TestUnmatchedReject_TypedNilDeclarativeDoesNotPanic` pins it. Verified by
      reverting both the guard and the nil-check: the test fails with the panic.

    `principalPropertyLookupEnabled` and `effectiveUnmatchedPrincipal` also gained
    nil-receiver handling, so the whole predicate chain is safe rather than only its
    entry point.
status: addressed
---

## Resolution

Two changes, defense in depth:

- `Declarative.Policy()` is now nil-receiver safe, returning a nil *Policy. The
  Policy predicates each handle a nil receiver, so a typed-nil now gives the
  same "no policy" answer as an absent one instead of panicking.
- `TestUnmatchedReject_TypedNilDeclarativeDoesNotPanic` pins it. Verified by
  reverting both the guard and the nil-check: the test fails with the panic.

`principalPropertyLookupEnabled` and `effectiveUnmatchedPrincipal` also gained
nil-receiver handling, so the whole predicate chain is safe rather than only its
entry point.
