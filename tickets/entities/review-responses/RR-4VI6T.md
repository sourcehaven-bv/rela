---
id: RR-4VI6T
type: review-response
title: AC1 regex for confirm() detection is sloppy
finding: 'Realistic risk near zero, but if writing the criterion at all, broaden it: grep for ''confirm(\|window\.confirm\|globalThis\.confirm'' across src/, excluding ConfirmModal.vue, ConfirmModal.test.ts, useConfirm.ts, useConfirm.test.ts.'
severity: minor
resolution: AC1 broadened to grep for 'window\.confirm\|globalThis\.confirm\|[^a-zA-Z]confirm(' across src/, with explicit exclusions for ConfirmModal.vue/test, useConfirm.ts/test.
status: addressed
---
