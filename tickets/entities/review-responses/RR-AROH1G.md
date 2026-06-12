---
id: RR-AROH1G
type: review-response
title: Manual-id_type prefixes are now gated too — scope expansion pinned deliberately
finding: 'The load gate applies to ALL declared prefixes, including id_type: manual types that declare one for type-routing only — a behavior change beyond the literal short-ID bug (reviewer leverage observation; the in-tree manual prefix MOD- passes).'
severity: minor
resolution: 'Decision made deliberate: a routing prefix outside the ID charset could never match a ValidateID-legal ID, so load rejection is strictly more honest. Pinned with TestParse_InvalidIDPrefixRejected_ManualIDType and documented in the bug entity.'
status: addressed
---
