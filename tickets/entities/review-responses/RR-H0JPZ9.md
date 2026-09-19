---
id: RR-H0JPZ9
type: review-response
title: resolve flattened a failed read gate into Resolved=false, turning "could not run" into a content-shaped violation
finding: 'Code review. validationgraph.resolve used `if err != nil || e == nil` and collapsed three distinct outcomes: store.ErrNotFound (a dangling reference), an ACL-hidden far end, and visibility.ErrReaderUnavailable (the read gate FAILED TO BUILD — the RR-GKCZO5 fail-closed sentinel, which internal/visibility deliberately made non-errors.Is-equal to ErrNotFound). The first two are correctly Resolved=false per the documented contract. The third is not: under a reader that gates rows and denies, a `min: 1` gate would report "has 0" — a content-shaped violation message — rather than reporting that the check could not run.'
severity: significant
resolution: 'resolve now switches: store.ErrNotFound returns an unresolved element (the edge is real and the bound must still decide); any other error is returned and propagated by RelatedEntities, so the constraint is reported as unevaluable rather than counted as zero. A nil entity with no error is treated as absent rather than panicking, with a comment saying it is a backend contract violation. validationgraph cannot import internal/visibility (arch-lint), so keying on store.ErrNotFound rather than on the visibility sentinel is what makes this robust to any reader.'
status: addressed
---
