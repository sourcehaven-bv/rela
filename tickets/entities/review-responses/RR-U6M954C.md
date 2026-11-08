---
id: RR-U6M954C
type: review-response
title: 422 range index parsed from prose
finding: 'The 422 detail is formatted as range N: ...; that is not a contract. Add a machine-readable field to the problem or isolate ranges.'
severity: significant
resolution: 'The server''s per-range problem answers (422 and 400) name the refused range in errors[].source.pointer as /ranges/<index>; httpremote parses that pointer into RangeError.Index. Without it the client uses the sole relation range of the request or fails with ErrProtocol. (Corrected for issue #2140: an earlier text named a range member that does not exist.)'
status: addressed
---
