---
id: RR-O7YJGC
type: review-response
title: Stores do not validate the relation tail grammar
finding: No backend runs ParseFace on k.FromFace in CreateRelation; fsstore builds a filename from it.
severity: minor
reason: Pre-existing and symmetric with entity faces which stores also do not validate. Face grammar is enforced at the boundary parsers (ParseRef and ParseFace). Store-level validation of entity faces and relation tails on write is tracked in TKT-5W4ISW.
status: deferred
---
