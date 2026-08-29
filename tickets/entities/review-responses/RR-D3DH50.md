---
id: RR-D3DH50
type: review-response
title: in/ne trim the filter side but not the property side
finding: matchesAnyCSV trimmed each comma member, so filter[tags][in]=' leading' does not match a property value ' leading' while filter[tags]=' leading' does. Same value, two answers depending on operator. Newly more reachable because the frontend routes multi-select through in.
severity: minor
reason: 'Pre-existing (the trim is inherited verbatim from the old code) and now much less reachable: a single selection stays on `=`, so the asymmetry only appears on a hand-written URL with leading/trailing spaces inside a comma list. Trimming both sides would silently change which rows match for existing scalar filters — an unannounced data-visible change riding on a bugfix. The trim is now documented in filterSetValues so the asymmetry is at least discoverable. Belongs with the operator-semantics reconciliation in TKT-UTJ24Z.'
status: deferred
---
