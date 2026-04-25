---
id: RR-LWO4N
type: review-response
title: Dead defensive check in editEntity
finding: DocumentView.vue editEntity has 'if (!cfg) return' but the button is gated by v-if="editConfig", so the early return is unreachable. Either drop or annotate why it exists.
severity: nit
resolution: Dropped the `if (!cfg) return` and replaced it with `editConfig.value!` (non-null assertion) plus an updated comment explaining why the button gating makes the assertion safe.
status: addressed
---
