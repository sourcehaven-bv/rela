---
id: RR-7F6NM9
type: review-response
title: File backend concurrency and durability are unspecified
finding: 'The plan names filecomments as the DEFAULT backend (''one file per target'') but specifies nothing about concurrent writes or partial writes. Two concrete gaps: (1) Two simultaneous POSTs to the same target both read the file, append, and write — classic lost update. The plan lists this under edge cases (''both comments survive'') but assigns no mechanism. fsstore''s answer is store.Store.Tx as a write mutex (CLAUDE.md, DEC-8UIL0); the comment store needs its own equivalent and it must be stated, because commentstest.RunAll can only pin behaviour the interface actually promises. (2) A crash mid-write truncates the file and loses every comment on that target, not just the new one. The fs tier writes whole files; an atomic write-temp-then-rename is the standard mitigation and should be specified rather than left to the implementer. Note this is more severe than the equivalent risk in fsstore, because one file holds N comments rather than one entity.'
severity: significant
resolution: 'The interface contract now states both explicitly: writes to one target are serialised (the store.Store.Tx role, DEC-8UIL0) with AC11 pinning that two concurrent adds both survive, conformance-tested across backends; and the file backend writes atomically via temp-file + rename, because one file holds N comments so a truncated write would lose a whole thread rather than one record.'
status: addressed
---
