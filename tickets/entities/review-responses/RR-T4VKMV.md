---
id: RR-T4VKMV
type: review-response
title: relType oracle still hand-modeled — same staleness class just fixed for IDs
finding: The relType validity check remained the inline ""/-- model while backends validate inline; the next tightening would produce false fuzz failures again. pgstore's feed.go even documents an assumption (no control chars in relation types) the validation didn't enforce.
severity: significant
resolution: 'Extracted storeutil.ValidateRelationType (ID-equivalent rules: empty, --, path separators, control chars); all three backends now use it, fixing the latent feed.go assumption and the fsstore ''/''-in-relType directory hazard; the fuzz oracle delegates to it directionally.'
status: addressed
---
