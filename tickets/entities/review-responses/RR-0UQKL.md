---
id: RR-0UQKL
type: review-response
title: Display-name fallback duplicated across three components, with subtle inconsistency
finding: RelationCards.vue:130 and LinkExistingModal.vue:118 both have a title || ... || id fallback — note LinkExistingModal also considers properties.name, which RelationPicker does not. A name-only entity will render its id in RelationPicker but its name in LinkExistingModal. Out of scope per the ticket, but worth flagging for a future shared formatEntityLabel util in src/utils/.
severity: minor
reason: Out of scope per ticket and confirmed by user (only RelationPicker.vue, not RelationCards.vue or LinkExistingModal.vue). Worth a follow-up ticket to introduce a shared formatEntityLabel util in src/utils/ that also considers properties.name.
status: deferred
---
