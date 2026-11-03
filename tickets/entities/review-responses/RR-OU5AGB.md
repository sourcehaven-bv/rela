---
id: RR-OU5AGB
type: review-response
title: The query-plan pins test copies of the SQL, not the production statements
finding: 'internal/channels/sqlitechannels/reads_explain_test.go:15,21 and internal/store/pgstore/channel_reads_explain_test.go:37 hand-copy unreadSQL, the Reads statement and the ListReads statement, with the comment "keep it in step". If someone edits sqlchannels/reads.go (for example, drops `m.counts = 1` so the partial index no longer matches, or reorders the ORDER BY), both tests stay green. That is the regression these tests exist to catch. Also, the SQLite test pins a SetRead-shaped count and the window, but not the Reads chunk statement with the row-value IN (VALUES ...), which is the hot path for the list and the inbox. Fix: export the statement builders from sqlchannels (for example, an ExplainStatements(n int) map[string]string that returns the real text with n key tuples), use them in both tests, and add the Reads chunk to the SQLite pin.'
severity: minor
resolution: sqlchannels.ReadSQL exports the production statements; pg and sqlite plan tests EXPLAIN them with arguments; sqlite also pins the Reads chunk
status: addressed
---
