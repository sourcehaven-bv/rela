---
id: RR-IWXMDW
type: review-response
title: 'Invariant: ETag = HashEntity(raw), body = redact(raw), same snapshot but not the same bytes'
finding: 'Implementer footgun (design-review S4). The ETag must be canonical.HashEntity on the RAW store entity; the wire body is the SEPARATELY redacted copy. If someone ''tidies'' this to hash the redacted body, every client''s If-Match breaks the instant any hidden field exists: client hashes redacted body -> server recomputes hash from full record -> permanent 412. canonical.HashEntity already ignores the per-reader Inaccessible set by design (canonical.go:63), which is what keeps the token reader-independent. Must be stated as a load-bearing invariant so it survives future refactors.'
severity: minor
resolution: 'Invariant enforced + documented at the fetch site (sync_handlers.go handleSyncGet): ETag = canonical.HashEntity/HashRelation over the RAW record; the wire body is the separately redacted copy. Comment states ''never hash the redacted body''. Pinned by TestSync_RedactedFetch_RoundTripsUnderReturnedETag (fetch redacted body, push back under returned ETag -> 200).'
status: addressed
---

## Finding (design-review S4)

Load-bearing invariant to write into the design and a code comment at the fetch
site:

> The sync ETag is `HashEntity(raw store record)`. The wire body is
> `redact(raw)`. They are computed from the **same snapshot** but are **not the
> same bytes** — the hash is reader-independent (full record), the body is
> reader-specific (redacted). Never hash the redacted body.

Pin with the existing round-trip test extended: fetch a redacted body, push back
under the returned ETag → 200 (proves the server's full-record recompute matches
despite the client never seeing the hidden fields).
