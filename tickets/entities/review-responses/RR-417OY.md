---
id: RR-417OY
type: review-response
title: EntityDetail closes delete modal on error, losing context
finding: 'EntityDetail.vue deleteEntity() finally block resets both deleting=false AND showDeleteConfirm=false. On error, the modal vanishes and only a transient toast remains — the user loses context of what they were trying to delete and can''t retry without navigating back. EntityList.vue has the correct behavior (keeps modal open on error). The two views are inconsistent. Fix: keep modal open on error in EntityDetail as well.'
severity: significant
resolution: Moved showDeleteConfirm.value = false out of the finally block into the try-block's success path in EntityDetail.vue deleteEntity(). On error the modal now stays open (with busy cleared), matching EntityList behavior. The user retains context and can retry or cancel.
status: addressed
---
