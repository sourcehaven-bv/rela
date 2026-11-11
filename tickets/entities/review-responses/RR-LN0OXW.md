---
id: RR-LN0OXW
type: review-response
title: 'Security: extra lookup on the miss path is a timing signal'
finding: 'Security review: the miss path does extra work for an id that was retired, so response timing could tell a retired id from one that never existed.'
severity: minor
resolution: Resolver.Successor asks for the successor before any gated read, so every miss does the same one Successors lookup. Only a retired id goes on to one gated read of its successor. The remaining signal says only that an id was once used, not anything about a row the reader cannot see; the uniform 404 body and status are unchanged (pinned by the hidden-successor parity tests).
status: addressed
---
