---
id: RR-YPC393
type: review-response
title: Pending label 'Sending' shows while no request is in flight
finding: 'RlChannelDecisions.vue:62-63 gives the pending key the `loading` and ''Sending'' treatment. useSessionDecisions.ts:144 sets that key before the confirm dialog (line 150) and before the discard-edit dialog and composer select in openForm (lines 106-117). For the whole time a confirm dialog is open, the button reads and announces ''Sending'' although nothing has been sent. A form decision with no edit also flashes ''Sending''. AC 7 requires the lock before any await, not the pending visual. Fix: keep `deciding` as the lock, which already disables the other buttons through DECISIONS_BLOCKED_KEY-style blocking, and set the per-key pending only right before `d.post`.'
severity: minor
resolution: The per-key pending (Sending) is set only around the post; while a dialog is open the deciding flag adds DECISIONS_BLOCKED_KEY, so the other buttons are blocked without any button reading Sending.
status: addressed
---
