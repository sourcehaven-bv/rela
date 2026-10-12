---
id: RR-NH28MV
type: review-response
title: Round 2 test gaps
finding: 'Round 2 listed missing tests: denied remove, non-zero RecordID, per-face counting, CalDAV stale edit and hidden member, and fallback with denied remove.'
severity: minor
resolution: Fixed in 3b2b0bdd8 and 2869cf91e. Added TestReplaceRelations_DeniedRemoveWritesNothing, TestReplaceRelations_PassesLineageIDToRecorder, TestReplaceRelations_ContentScopeCountsPerFace, TestDynamicCollections_StaleEditDoesNotUndoMove, TestDynamicCollections_MissingMembershipDeleteIsNotAnOracle and TestPatchRelations_DisallowedTypeWithDeniedRemoveWritesNothing.
status: addressed
---

Review finding R2-12.
