---
id: RR-FOCRST
type: review-response
title: '"The dialog''s focus restore is unreachable, and Escape does nothing while loading"'
finding: EntityDetail mounts DuplicateModal under `v-if="showDuplicateModal && ..."` AND binds `:show="showDuplicateModal"`. Closing sets the flag false, which unmounts via v-if before the watch can observe the transition, so the `!isOpen && wasOpen` branch that restores previouslyFocused never runs. Focus is never returned to the triggering button — an accessibility defect for a dialog. Separately, focus is set AFTER `await loadRelations()`, so during the fetch focus is still on the button behind the overlay and Escape (bound to the dialog element, correctly) does nothing in the one phase where the user most wants out. There is also no focus trap.
severity: significant
resolution: Replaced the show-prop watch with onMounted/onBeforeUnmount and removed the redundant :show binding, so the component's lifetime is the dialog's. Focus is taken before the fetch rather than after, so Escape works during loading. Pinned by 'returns focus to the element that opened it', which fails against the watch version.
status: addressed
---

## Suggested resolution

Pick one gate (v-if or :show, not both), focus the dialog before the fetch rather than after, and restore focus on unmount.
