---
id: RR-NBMXKM
type: review-response
title: Empty-entry rendering conflicts with the empty-group rule
finding: views_handler.go:258 drops empty groups; AC7 would show Projects / None, including to a principal denied the type.
severity: minor
resolution: 'An entities: entry with no rows renders nothing. A group whose items all render nothing hides its heading, client-side, consistent with the server rule. AC7 rewritten.'
status: addressed
---
