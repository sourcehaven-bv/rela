---
id: RR-7D2FJZN
type: review-response
title: 'Use one ASCII fold helper once #2245 lands'
finding: 'MatchesID uses strings.EqualFold and relies on the byte-length slice to stay ASCII-only, while #2245 adds an unexported ASCII fold helper in internal/entity.'
severity: nit
reason: 'The helper is not in this base. The comment on MatchesID explains why EqualFold is ASCII-only here. Unifying is a small change after #2245 merges.'
status: deferred
---
