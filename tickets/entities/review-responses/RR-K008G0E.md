---
id: RR-K008G0E
type: review-response
title: Take remote on a row-level refused entry writes every mapped field
finding: A refused push resolved to remote makes the next pull write all fields the push tried to send. Edit rights checked on one field are not enough.
severity: significant
resolution: The adapter supplies the local field names behind RefusedFields; take remote on a row-level entry needs every one of them writable. Digests never go on the wire.
status: addressed
---
