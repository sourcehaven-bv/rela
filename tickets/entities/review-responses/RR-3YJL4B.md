---
id: RR-3YJL4B
type: review-response
title: Relation filter compares unredacted neighbour titles
finding: 'api_v1.go: relation filter keys skip the field gate, but matchRelationFilterMany compared DisplayTitle on raw neighbour headers from servedIDsErr. When the neighbour type''s display property is hidden from the principal, filter[assignee]=<guess> confirms the hidden title.'
severity: significant
resolution: 'Verified the code reviewer was right: newVisibleReader builds its resolver with visibility.NopRedactor, so ResolveIDsErr returns raw titles. matchRelationFilterMany now takes the app redactor and redacts the served neighbour headers once (redactHeaders: prime traversals for the batch, then RedactHeader per header) before DisplayTitle. TestListFieldGate_RelationFilterComparesRedactedNeighborTitle fails without the fix and passes with it.'
status: addressed
---
