---
id: RR-YV817D
type: review-response
title: No configtest conformance harness for Loader implementations
finding: internal/config is about to grow a second Loader (SQLite), and the point of the seam is that the two are interchangeable. internal/store/storetest and internal/state/statetest are the established pattern, and CLAUDE.md mandates a conformance suite for new store implementations. A configtest.RunLoaderTests covering absent-dir, not-a-directory, sorting, non-recursion, symlinks and the ErrNotExist contract would have caught the OsFS/MemFS divergence the moment it appeared.
severity: minor
reason: Agreed in principle, deferred to TKT-S1EVV7 where the second implementation lands. Writing the harness now would mean designing a cross-implementation contract against exactly one implementation, and the FS-specific cases (symlinks, MemFS's directory map) do not all translate to a table-backed loader — the harness would be shaped by FSLoader's incidentals rather than by the contract. The divergence it would have caught is fixed and pinned by a direct test in the meantime.
status: deferred
---
