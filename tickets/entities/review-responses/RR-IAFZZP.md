---
id: RR-IAFZZP
type: review-response
title: Unbounded backlink result sets
finding: MentionSources returned every source with no paging; every view of B would load and filter all mentioners in Go, so mass mentions degrade reads and a count before filtering is an oracle.
severity: significant
resolution: 'As built: store.MentionIndex.MentionSources is keyset-paged by (id, face) in every backend. visibility.Resolver.MentionedBy reads it in batches of 500, gates each batch with one header read, and stops at limit or at a 10,000-row scan budget. REST and Lua take limit and cursor; CLI and MCP cap at 50. truncated and next_cursor are computed after filtering, and the cursor is a served id only. The GraphQuery-predicate design was dropped during implementation (see PLAN-2CMYVT As built).'
status: addressed
---
