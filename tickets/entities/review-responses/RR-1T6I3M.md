---
id: RR-1T6I3M
type: review-response
title: comments.Store has no single-comment read, so Service.Get pulls the whole thread
finding: Service.Get calls List and linear-scans in Go to find one comment, so every Update and Delete authorization check pulls up to MaxPerTarget rows with their bodies to read one author field. That is the per-row-lookup shape CLAUDE.md's collection-reads rule exists to prevent. Predates this ticket — it was the only option when filecomments was the sole backend and a thread was one document — but now that two backends can push it into SQL, a Get(ctx, target, id) on the interface is cheap.
severity: minor
reason: 'Deferred as a follow-up ticket rather than done here, because it is an interface change and TKT-OGTVJW''s AC1 freezes comments.Store ("no interface change, which is the constraint TKT-FIO205 set"). Doing it now would also oblige all four backends and the conformance suite to move in the same commit as two critical data-loss fixes, which is the wrong thing to bundle. Worth doing: the win is real on the database backends and it costs filecomments nothing, since a thread is one document there either way.'
status: deferred
---
