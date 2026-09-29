---
id: RR-O7YJGC
type: review-response
title: Stores do not validate the relation tail grammar
finding: No backend runs ParseFace on k.FromFace in CreateRelation; fsstore builds a filename from it.
severity: minor
reason: Pre-existing and symmetric with entity faces, which stores also do not validate. Face grammar is enforced at the boundary parsers (ParseRef, ParseFace). Store-level face validation for entities and relations belongs with the malformed-face address tests (A9, PR 6).
status: deferred
---
