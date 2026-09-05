---
id: RR-ND21P3
type: review-response
title: Client ceiling silently deleted state-shaped grants — and the narrower ceiling was the destructive one
finding: |-
    VERIFIED by execution. `filterTypes` (ceilingcompile.go:283) intersected the role's grant list against the ceiling by comparing WHOLE LITERALS. A state grant is the literal "page@draft", which never equals "page" and is not "*", so under an allowlist ceiling it failed the permits check and was deleted from the role:

      allowlist update:[page]  -> Update after clamp = [page]              <- page@draft GONE
      allowlist update:[*]     -> Update after clamp = [page@draft page]
      deny_update:[other]      -> Update after clamp = [page@draft page]

    Two things make this worse than a plain fail-closed.

    1. THE INVERSION. The wildcard branch returns early, so the BROAD ceiling `update: ["*"]` KEPT the state grant while the NARROWER `update: ["page"]` destroyed it. A grant surviving the broad ceiling and dying under the specific one is the same 'widening by being made more specific' inversion grantsVerb's own godoc warns against.

    2. THE DENIAL BLAMED THE WRONG LAYER. Result was Allow=false with RuleKind="role-grant" and 'no role grants update on type "page"'. But the ceiling PERMITS update on page and the role HOLDS update on page@draft. An operator reading the audit log is sent to inspect an acl.yaml whose grant is plainly present. decideFromAttrs' own comment says the client-ceiling vs role-grant distinction exists precisely so that cannot happen.

    Invisible before this PR because grantsVerb skipped state grants anyway — so this PR is what made the latent clamp bug reachable, which puts it in scope. It would have bitten the copy kernel first: TKT-C1XUA8's whole premise, 'published is writable only via copy definitions', is expressed as a state grant, and under any allowlist or scope-grant ceiling that grant would have evaporated.
severity: critical
resolution: |-
    filterTypes now matches on the TYPE half (`v.permits(grantTypeOf(t))`), preserving the entry. A ceiling names entity TYPES, so it clamps at type granularity and lets GrantsVerbOnState adjudicate the face.

    Still fail-closed: a DENIAL reaches every face regardless, because permitsVerb runs before the role loop against the subject's bare type. Pinned by TestCeilingClampsOnTypeNotLiteral across four ceiling shapes, asserting BOTH the verdict and the RuleKind — a denial must name the layer that actually caused it. Verified it bites: reverting to whole-literal matching reproduces the exact misleading denial.
status: addressed
---
