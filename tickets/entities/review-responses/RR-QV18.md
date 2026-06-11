---
id: RR-QV18
type: review-response
title: Attributed role is nondeterministic (map-iteration order) (S3)
finding: 'effectiveRoles returns roles by ranging a map (random order); observeDeny records ''the last role that observed a deny.'' When a path is denied by multiple roles, which role is attributed varies run to run, making the audit role= suffix flaky. Fix: sort the effective-role slice and define a deterministic attribution-selection rule (first denying role in sorted order).'
severity: significant
resolution: effectiveRoles now sort.Strings the role slice; dimension/option observeDeny keep the FIRST denying role (was last-writer). relationAccumulator.recordDeny already kept-first. Attribution is now deterministic (first denier in sorted role order). Pinned by TestResolver_MultiRoleDeny_DeterministicAttribution (20 iterations, asserts stable 'default' wins over 'zeta').
status: addressed
---
