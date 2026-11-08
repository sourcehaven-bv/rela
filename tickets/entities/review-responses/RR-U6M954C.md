---
id: RR-U6M954C
type: review-response
title: 422 range index parsed from prose
finding: 'The 422 detail is formatted as range N: ...; that is not a contract. Add a machine-readable field to the problem or isolate ranges.'
severity: significant
resolution: The server's per-range problem answers now carry a range member (v1.Error.Range) for 422 and 400; httpremote maps it to RangeError.Index. Without it the client uses the sole relation range of the request or fails with ErrProtocol.
status: addressed
---
