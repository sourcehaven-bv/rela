---
id: RR-23MD7
type: review-response
title: Bulk action confirm path silently drops errors (executeAction never throws)
finding: 'EntityList.vue''s onRequestConfirm wraps executeAction in onConfirm, but useListActions.executeAction uses Promise.allSettled and surfaces errors via uiStore.error — it never throws. So even when 100% of bulk-action writes fail, useConfirm settles with success and the modal closes. The ''modal stays open on error'' semantics work for the per-row delete (which throws) but not for bulk actions. Fix: simplify the path — call executeAction outside onConfirm and let it run in background; the modal close-on-confirm is fine since executeAction toasts its own results.'
severity: critical
resolution: 'EntityList.vue: bulk action path now calls executeAction outside onConfirm. requestActionConfirm awaits confirm() (no callback), then on ok calls executeAction without awaiting. The action runs in background and toasts its own results via uiStore.error / script-error dialog. Comment in code documents the rationale (executeAction uses Promise.allSettled and never throws).'
status: addressed
---
