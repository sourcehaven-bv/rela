---
id: RR-73CB
type: review-response
title: Docs and comments over-promised the fail-closed guarantee
finding: 'The commit deleted the honest RR-TPATBK "known limitation" note and replaced it with docs/comments asserting "every grant that would need untrusted subject-world state is hidden" — which was false while the role-resolution path (RR-73CA) still leaked. The WithHistoricalSubject doc also claimed has_role "degrades to globals-only safely for a deleted entity", true only for deleted (no edges), not drifted-but-live entities.'
severity: significant
resolution: 'With RR-73CA fixed the guarantee is now true. Updated docs/acl-security.md (via docs-project source, regenerated) and the WithHistoricalSubject / FieldVerdicts / historical.go comments to describe the COMPLETE neutering: outgoingCounts, globals-only role resolution, and the type-level closed-world — with the stated invariant matching the implementation.'
status: addressed
---
