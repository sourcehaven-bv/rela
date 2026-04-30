---
id: RR-4PMY1
type: review-response
title: Stale comment in EntityList.test.ts describes deleted plumbing
finding: Comment block at EntityList.test.ts:25-30 describes the old pendingDelete -> ConfirmModal -> confirmDelete flow that was removed. Update or delete.
severity: minor
resolution: Updated comment in EntityList.test.ts to describe the new useConfirm-based wiring.
status: addressed
---
