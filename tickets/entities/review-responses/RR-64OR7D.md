---
id: RR-64OR7D
type: review-response
title: Server-Timing query count is a hidden-row oracle until batching lands
finding: Before D.1 replaces per-row neighbour loads, the number of SQL statements a list page issues is a function of how many neighbour rows exist, including rows the principal cannot read (the per-neighbour GetEntity runs before filterVisible). Exposing that count in a response header to every client turns the existing internal cost into a count oracle over hidden rows, contrary to the row-level rule in CLAUDE.md. Even after batching, exposing DB timings by default adds a channel that was previously only observable as coarse wall time.
severity: significant
resolution: 'Plan changed: the Server-Timing header and the per-request log record are emitted only when Debug logging is enabled (rela-server -verbose), the same condition that already gates the pgx tracer. Nothing reaches the wire by default, so the ordering of PRs no longer matters for this channel. Documented in docs/acl-security.md beside the timing-exposure note.'
status: addressed
---
