---
id: TKT-CICJSN
type: ticket
title: 'MCP analyze_cardinality: delete the fifth copy, call the consolidated analysis service'
kind: refactor
priority: high
effort: s
status: backlog
---

Follow-up from TKT-RNBLAC's code review. TKT-RNBLAC consolidated the four
tracer/analysis cardinality functions into one `checkCardinality` with real
error propagation — but `internal/mcp/tools_analysis.go` keeps a FIFTH
hand-rolled implementation (`checkCardinalityBound`, ~line 103) that is now
strictly worse than the shared one:

- `count, _ := st.CountRelations(...)` swallows backend errors → an outage
manufactures `min` violations out of count 0 (the exact bug §12.6 fixed).
- `if err != nil { break }` on the `ListEntities` iterator silently truncates
the scan — partial results presented as complete.
- Divergence risk: with one good implementation and one bad, every future
change (notably TKT-9KZGJO's `must_hold_in` world-awareness) must land twice or
the MCP surface silently reports different violations than CLI/validate.

Fix shape: delete `checkCardinalityBound` and have the MCP tool call the
consolidated `analysis` service (`CheckCardinality`), mapping its violations
into the MCP payload and failing the tool call on error — same contract the
three CLI surfaces now have. Gate on TKT-RNBLAC being merged (#1381).

Also blocks-cleanly-before TKT-9KZGJO (Step 5): world-awareness must find ONE
cardinality implementation when it lands.
