---
id: RR-VWDRY
type: review-response
title: Group-by-type-then-PermitsReadMany now exists in four places, and the two new copies were the ones missing the face half
finding: |-
    visibleHeaderIDs, filterVisible, readableViewIDs and the new block in loadViewEntities all implement group-ids-by-type then PermitsReadMany. Three apply faceReadable; the two new ones did not — and that divergence WAS the RR-VW2FAC finding. The reviewer's argument: one shared helper applying both halves would make the face check impossible to forget.
severity: minor
status: wont-fix
reason: |-
    The divergence is fixed (RR-VW2FAC) and pinned by a lint tripwire that requires faceReadable at both new sites, so the concrete failure mode is closed.

    Not extracting a shared helper, for two reasons. First, the four call sites differ in what they start from and what they produce: two start from headers, one from loaded entities, one from bare ids needing a header scan; two return filtered slices, one mutates a map, one returns a verdict set. A helper covering all four would take a shape-converting adapter at every call site, which is more code at each site than the loop it replaces.

    Second, the two visibility-package sites and the two dataentry sites sit on opposite sides of a package boundary that exists deliberately (read-out wrappers vs. the traversal that must stay raw mid-walk). Hoisting a shared gate across it would create exactly the kind of cross-cutting helper CLAUDE.md warns about.

    The load-bearing part of the suggestion — "the face half must not be forgettable" — is implemented as the lint tripwire instead. If a fifth call site appears, revisit.
---

## Context

Found by code review of the BUG-9Z20WH fix.
