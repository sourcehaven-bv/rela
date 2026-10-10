---
id: RR-EF4ZSQ
type: review-response
title: Compressed raw body stored compressed
finding: A raw body with Content-Encoding gzip was stored compressed under the plain name.
severity: nit
resolution: A non-identity Content-Encoding on a raw body answers 415 unsupported_content_encoding; declared in the spec and API reference; table-tested.
status: addressed
---
