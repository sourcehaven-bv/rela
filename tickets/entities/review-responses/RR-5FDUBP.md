---
id: RR-5FDUBP
type: review-response
title: resolveOutOfPageLinks read props.value while selectedEntities rendered effectiveValue
finding: The resolve computed its missing-id set from props.value, but selectedEntities renders effectiveValue. Identical for an outgoing picker today, so it worked, but the two were coupled by coincidence rather than construction — a future relaxation of the isIncoming guard would silently desynchronise them.
severity: significant
resolution: Switched the resolve to read effectiveValue, the same source selectedEntities renders, with a comment naming why. One-word change; removes the coincidence.
status: addressed
---
