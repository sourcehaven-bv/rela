---
id: RR-N7C2ZV
type: review-response
title: onScreenFace duplicates textVars.face's resolution expression, keeping a comment's promise by hand
finding: 'EntityDetail.vue:935 and :959 both compute `servedFace.value || (typeDef.value?.bare_face ?? '''')`. The onScreenFace comment explicitly promises the two agree (''Same resolution textVars.face uses, so a note and the {face} inside it always mean the same face'') — a promise currently kept by two copies staying in sync by hand. onScreenFace is declared after textVars, so it cannot simply be referenced; moving it above and writing `face: faceLabelOf(onScreenFace.value)` makes the promise structural rather than aspirational.'
severity: minor
resolution: Moved onScreenFace above textVars and changed textVars.face to `faceLabelOf(onScreenFace.value)`, so the two now share one expression instead of two hand-synced copies. The comment's promise that a note and the {face} inside it always mean the same face is now structural. Also noted in the comment that '' arises only for a faced type with no bare_face, where no face genuinely is on screen — the case RR-BYKCHQ now pins.
status: addressed
---
