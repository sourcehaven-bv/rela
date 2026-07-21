---
id: RR-CHW2AA
type: review-response
title: jwtauth is a declared arch-lint leaf; the plan's jwtauth->principal dependency fails CI
finding: 'The plan''s Dependencies line states "jwtauth -> principal (new, one-way)". .go-arch-lint.yml:469-473 declares jwtauth with `canUse: [jwtlib]` — canUse, not mayDependOn — i.e. a leaf with zero internal dependencies, and the comment says so deliberately: "Leaf: verifies signed JWTs against a JWKS. No internal deps; only the JWT vendor libs. The dataentry resolver consumes it via a local interface." Adding the edge fails `just arch-lint`. The dependency is also unnecessary: AssertionClaims is jwtauth''s own type and dataentry maps it to principal.Principal at router.go:405, exactly as the existing subjectVerifier seam keeps dataentry from importing jwtauth. The plan''s step 3 already describes the correct wiring via the webhookVerifierAdapter pattern; only the Dependencies line contradicts it.'
severity: significant
resolution: 'Confirmed: .go-arch-lint.yml:469-473 declares jwtauth with `canUse: [jwtlib]` (canUse, not mayDependOn) with a comment stating the leaf status is deliberate. The claimed jwtauth->principal edge is dropped from the plan''s Dependencies — it was never needed. AssertionClaims stays jwtauth''s own type; the mapping to principal.Principal happens at the dataentry construction site via the principal.Verified constructor, and cmd/rela-server adapts between them exactly as webhookVerifierAdapter already does (main.go:244-252). jwtauth remains a leaf. `just arch-lint` is part of the pre-PR check per CLAUDE.md, so this is verified rather than assumed.'
status: addressed
---

## Finding

`.go-arch-lint.yml:469-473`:

```yaml
  jwtauth:
    # Leaf: verifies signed JWTs against a JWKS. No internal deps; only the JWT
    # vendor libs. The dataentry resolver consumes it via a local interface.
    canUse:
      - jwtlib
```

`canUse`, not `mayDependOn` — a declared leaf with zero internal dependencies,
and the comment says so on purpose. The plan's claimed `jwtauth → principal`
edge fails `just arch-lint`.

## Resolution

Drop the edge; it is not needed. `AssertionClaims` is jwtauth's own type, and
`dataentry` maps it to `principal.Principal` at the construction site
(`router.go:405`) — the same discipline the existing `subjectVerifier` seam
(`router.go:362`) uses to keep `dataentry` from importing `jwtauth`. The plan's
step 3 already describes this correctly via the `webhookVerifierAdapter` pattern
(`cmd/rela-server/main.go:244-252`); only the Dependencies line contradicts it.

Related: see the finding on widening the `subjectVerifier` seam — if widening
means returning a `jwtauth.AssertionClaims`, `dataentry` imports `jwtauth` and
the seam's whole purpose is lost.
