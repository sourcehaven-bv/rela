---
id: RR-34KJWO
type: review-response
title: Pending buttons looked fully enabled while silently swallowing clicks
finding: |-
    The only disabled styling in the app is `.btn:disabled { opacity: 0.6; cursor: not-allowed }` (App.vue:430). Nothing styled `[aria-disabled='true']`. So a pending PendingButton kept full opacity, kept `cursor: pointer`, kept its hover and active states — and silently ate every click via preventDefault.

    That is a WORSE affordance than what it replaced: the old `:disabled="saving"` at least greyed the control. And during the pre-delay window the label has not swapped yet, so there is zero feedback that the click was consumed — the user clicks, the button visually depresses on :active, nothing happens, no explanation.

    This compounded on SettingsView's logo Upload button, whose condition genuinely changed from `:disabled="!stagedLogo || uploadingLogo"` to `:pending="uploadingLogo" :disabled="!stagedLogo"`. Mid-upload the sibling 'Choose image' button stays greyed (it still uses native disabled) while Upload looks bright and live but does nothing — an inconsistency that reads as a bug. On a fast upload, which is the common case per this ticket's own operating model, showPending never fires at all.
severity: significant
resolution: |-
    `.btn[aria-disabled='true']` now mirrors the `:disabled` rule (opacity 0.6, cursor not-allowed), and hover/active feedback is suppressed for it — those imply the click will do something.

    Deliberately written as a separate selector rather than folded into `:disabled`, since decoupling from native disabled is the entire point of the aria approach: the control must stay focusable so a keyboard user is not stranded mid-interaction.

    This also resolves the SettingsView Upload inconsistency: the pending Upload button is now visually distinguishable from an idle one, and consistent with its greyed sibling.
status: addressed
---
