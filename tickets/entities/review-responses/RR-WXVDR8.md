---
id: RR-WXVDR8
type: review-response
title: Comments in the migration asserted things that were not true
finding: 'Three claims in the new code did not match reality. (1) 0015_comments.sql said the thread index makes List "an index range scan per target rather than a sort"; EXPLAIN shows the planner prefers the narrower prefix index and sorts, because a thread is capped at MaxPerTarget and sorting a few hundred rows is cheaper than the wider index''s I/O. (2) The same file said target_type is carried because "the read gate needs the type to resolve a verdict"; nothing in the backend reads the column. (3) pgcomments.DBTX declared Begin and nothing called it, which is precisely what CLAUDE.md''s minimum-interface rule warns against. A comment asserting a plan the database does not produce is worse than no comment: the next person tunes against it, or "fixes" a regression that was never real.'
severity: minor
resolution: 'Corrected all three. The index comment now says it CAN serve List pre-ordered, usually will not, and is insurance for the threads that would hurt — with the measurement behind it and a note to re-measure before calling a sort a regression. Verified by EXPLAIN against 20k rows: the planner does choose it unprompted, with no Sort node, once a single thread grows large. The target_type comment now says plainly that nothing reads it and that it is stored so a row is intelligible to an operator reading the table directly. Begin is no longer unused — the collision fix needs the transaction.'
status: addressed
---
