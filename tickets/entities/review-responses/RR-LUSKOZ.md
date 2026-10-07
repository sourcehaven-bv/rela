---
id: RR-LUSKOZ
type: review-response
title: '[security] Face-move collision merged a thread into another row''s thread'
finding: '[security] moveThreads ran before applyFaceMove''s collision check, so a refused move had already merged the stranded row''s thread into the different destination row''s thread, visible to that face''s readers.'
severity: significant
resolution: applyMoves now runs destinationHolds for the whole batch before any thread moves. Pinned by TestAdopt_CollisionLeavesCommentThreads, which fails with the pre-check removed.
status: addressed
---
