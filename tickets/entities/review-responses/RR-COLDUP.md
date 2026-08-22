---
id: RR-COLDUP
type: review-response
title: "viewCollectionTypes was a divergent hand-copy of ValidateConfig's map"
severity: minor
status: addressed
finding: "The new helper re-derived the collection-name to entity-type map that ValidateConfig also builds inline. It had already diverged once (omitting the implicit 'entry' collection, which silently skipped every source: entry section — caught during implementation). A second latent divergence remained: relName picked Follow first with FollowIncoming as fallback, while ValidateConfig assigns by sequential overwrite so FollowIncoming wins when an author sets both. Outcomes coincide today because determineTargetType branches on Follow first, so it was latent rather than live — but two copies of one derivation is exactly the duplication this ticket's own drift guard exists to prevent elsewhere."
resolution: "Aligned the precedence with ValidateConfig exactly (FollowIncoming overwrites Follow) and documented the helper as the single intended source, with a note to delete it in favour of ValidateConfig's building half if that is ever factored out. Pinned by TestViewCollectionTypes_IncludesEntry."
---
