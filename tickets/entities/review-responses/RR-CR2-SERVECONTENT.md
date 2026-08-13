---
id: RR-CR2-SERVECONTENT
type: review-response
title: No ETag/Last-Modified/Range, and every HTTP method returns a full body
finding: serveAsset uses w.Write, so POST/PUT/DELETE/OPTIONS all return 200 with the body, HEAD returns
  a body, and Range is ignored. RR-DR-ETAG already flagged that RR-CR-ETAG's deferral rationale ('the
  shell is 3.4KB and uncacheable-by-design') does not transfer to a static 200KB webfont, and required
  the deferral be re-opened or re-justified rather than inherited. Shipping with an acknowledging comment
  is not a re-justification.
severity: minor
status: deferred
resolution: DEFERRED to a follow-up ticket, explicitly rather than by inheritance. http.ServeContent would
  give conditional requests, Range and correct HEAD in one call and would let the manual ReadAll go -
  the reviewer's highest-leverage suggestion. Not taken here because it changes the read path this ticket
  just hardened (nested OpenRoot + size check) and would want its own traversal re-verification; folding
  it into a security-sensitive breaking change is how the careful part gets rushed. Recorded so the next
  person re-opens it deliberately.
reason: 'Deferred, not dropped. The fix (http.ServeContent) replaces the exact read path this ticket just
  hardened with a nested os.OpenRoot and a pre-read size check, so it needs its own traversal and containment
  re-verification. Bundling that into a breaking layout change is how the careful part gets rushed. The
  current behaviour is a performance and HTTP-correctness gap, not a security one: assets still serve
  correctly, just without conditional requests or Range. Follow-up ticket to be opened against FEAT-MD4N6Z.'
---

Raised by `/code-review` of the TKT-IWMETE implementation.
