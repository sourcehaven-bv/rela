---
id: RR-0HDG3I
type: review-response
title: worldAbsent guard in noticeNote is dead code, and the comment asserts it is load-bearing
finding: EntityDetail.vue:975 gates noticeNote on `worldAbsent.value || !onScreenFace.value`, but the template already gates the whole banner on `!worldAbsent` (line 1520). Removing ONLY the worldAbsent term from the computed leaves 67/67 tests passing — the behaviour is pinned exclusively by the template, so the computed's check is unreachable. The three-line comment above it explains why the guard exists, reading as a description of an active mechanism when it describes a no-op. commentlint cannot catch a semantically-dead guard. readOnlyNote has the identical redundancy, so this matches prior art, but that defends matching the pattern, not the pattern being right.
severity: significant
resolution: 'Verified the claim first: removing ONLY the worldAbsent term from noticeNote left 67/67 passing, confirming the term is inert and that my original mutation (which removed the computed AND template guards together) had misled me into thinking it load-bearing. Kept the term as deliberate defence-in-depth — dropping it would diverge from readOnlyNote, which carries the same redundancy — and rewrote the comment to say so plainly: ''The worldAbsent term is BELT-AND-BRACES, not the mechanism: the banner''s own v-if already gates on !worldAbsent, so removing this term changes nothing today. It is kept ... so a future caller rendering this note outside that banner inherits the silence.'' The prose no longer claims an inactive guard is the mechanism.'
status: addressed
---
