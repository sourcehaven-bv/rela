---
id: RR-TFATPO
type: review-response
title: deny_worlds cannot deny the default world through clamp — the ceiling axis has an inverted wildcard hazard
finding: |-
    VERIFIED, and it inverts the plan's own claim. dn37j2-plan.md §2.3 says 'there is no world wildcard by design, so the re-check is simpler'. Backwards.

    THE HOLE: the default world is IMPLICIT — it is the ABSENCE of a world grant, not an entry. So a role with `Worlds: []` means 'default world only'. A ceiling with `deny_worlds: [default]` intersects [] with anything and gets [] — which STILL means the default world. The denial is a NO-OP. That is fail-open, structurally identical to the gap permitsRead exists to close (readquery.go:33-35), and it cannot be fixed inside clamp at all.

    SECOND, WORSE: compiledCeiling.clamp (ceilingcompile.go:251-261) rewrites exactly five fields. A new RoleDef.Worlds that clamp merely IGNORES produces NO compile error and NO guard-test failure — ceilingguard_test.go scans for `policy.Roles[` access, not field coverage. That is precisely the fail-open the Q1 overrule was meant to prevent, and nothing structural catches it.

    UNDER-SPECIFIED COMPOSITION POINTS (§2.3 says 'participates like every other axis'; that is five separate tables in ceiling.go:140-290, three of which the plan leaves undefined):
    1. Narrows() (:153-159) — must add both, else a worlds-only baseline reads inert and A11 flags it.
    2. denySpellings() (:163-185) — needs a deny_worlds row.
    3. validateSpellings() (:198-227) — needs {"worlds","deny_worlds"}. Does deny_write (which fans out to three verbs at :216-241) interact with worlds? Should be no (worlds are read-side) but must be DECIDED.
    4. validateNoBlanks() (:246-280).
    5. expandClientAttenuation (:348-358) mutates ClientBaselines/ScopeGrants INSIDE Validate (policy.go:716); its ordering vs normalizeWorldGrants is unstated.

    FIX: specify permitsWorld as a MANDATORY post-check consulted by Request.PermitsWorld, not an optional 'wildcard re-check'. Decide explicitly how the implicit default world is denied — probably it can only be enforced at PermitsWorld, never through clamp. Add a field-coverage guard for clamp.
severity: significant
status: open
---
