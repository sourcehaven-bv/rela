---
id: RR-C9UH50
type: review-response
title: FaceMoved and post-commit races could lose or strand concurrent comments
finding: FaceMoved cleared the source with DeleteTarget, deleting a comment posted between List and delete; a comment posted after moveThreads but before the row Tx commit stayed at the old face.
severity: significant
resolution: FaceMoved deletes only the ids it listed and copied (Store.Delete, ErrNotFound tolerated). applyMoves runs moveThreads again after each committed batch. Pinned by the TestFaceMoved subtest 'a comment posted at the source during the move survives'.
status: addressed
---
