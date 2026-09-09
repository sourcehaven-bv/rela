---
id: RR-Q8SWR6
type: review-response
title: landing face arm pushes `@undefined` when the face is missing; unknown modes fall to written
finding: 'EntityDetail.vue landAfterCopy `case ''face''` pushes `${path}@${landing?.face}` unguarded; a hand-crafted or older response with `mode: face` and no face navigates to `ID@undefined`. The switch has no `written` arm so an unknown mode silently behaves as written. Guard the face and make the arms explicit.'
severity: significant
resolution: landAfterCopy has explicit written/stay/world/face arms; a world or face arm with no name, and an unknown mode, reload in place rather than navigating to `@undefined` or passing for written. Two EntityDetail tests pin the guard and the unknown mode.
status: addressed
---
