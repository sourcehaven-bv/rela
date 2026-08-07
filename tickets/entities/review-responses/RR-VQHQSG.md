---
id: RR-VQHQSG
type: review-response
title: graph.go DOT cluster label capitalized the type id and appended 's'
finding: internal/cli/graph.go:111 built a DOT cluster heading as strings.ToUpper(entityType[:1])+entityType[1:] + "s" — the same English orthographic guess plus naive pluralization the decision removes, in a file the original survey missed. It also sliced a byte ([:1]), so a non-ASCII type name produced mojibake. The entity def was already fetched three lines below for def.Color, so the authored label was in hand and unused.
severity: minor
resolution: 'Now resolves the cluster heading from the entity def: LabelPlural, else Label, else the raw type name, passed through escapeLabel. Guarded on def.Label being non-empty rather than calling GetLabelPlural(), because that accessor falls back to Label+"s" and yields a bare "s" when Label is unset (a real case — the same trap already hit the rela-desktop scaffolder). Existing graph tests assert the cluster ID, not the heading, and still pass. Found by the rewritten behaviour-based lint test, which is the point of that guard.'
status: addressed
---
