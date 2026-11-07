---
id: RR-3HN68CI
type: review-response
title: Redacted path needs full bodies for the view hash
finding: Computing the view hash in Go for readers with redacted fields reads and formats every body in scope on each sync (against collection-reads.md).
severity: significant
reason: RES-G8CYB1R chose to compute the redacted projection in Go one page at a time. Keeping the view hash equal to the stored content_hash is what lets unredacted readers use the stored column; a body-digest encoding would change every stored content hash and version dedup. The cost belongs to step 4 of FEAT-CMVZQFI (redacted path) which can add a stored body digest if measurements need it. Noted in the package doc.
status: deferred
---
