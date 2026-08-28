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
status: addressed
resolution: 'RESOLVED. serveAsset now delivers via http.ServeContent, which brings conditional requests,
  Range and correct HEAD semantics in one call. A new openCustomEntryFile returns a live *os.File plus
  modtime/size instead of bytes; the byte-returning openCustomEntry stays for the shell-injection path,
  which genuinely wants bytes and would be the wrong shape with a Close-me handle.


  ETag is modtime+size, deliberately not a content hash: hashing would read the whole file on every request,
  which is the exact cost ServeContent was adopted to avoid and a read amplifier on this unauthenticated
  route. http.FileServer makes the same trade. Cache-Control stays no-cache, so an operator edit is still
  visible on reload - it now costs a bodiless 304 rather than a full re-transfer.


  Two properties had to survive the rewrite and are pinned by tests: the explicit Content-Type (ServeContent
  sniffs when it cannot infer one - the exact behavior nosniff exists to prevent) and the pre-read size
  gate (ServeContent would otherwise stream a file of any size).


  The deferral concern - that this replaces the read path TKT-IWMETE had just hardened and needs its own
  containment re-verification - was addressed rather than assumed away. openCustomEntryFile duplicates
  the containment chain because it cannot delegate to openCustomEntry, and a duplicated security check
  drifts, so: TestOpenCustomEntryFile_MatchesOpenCustomEntry asserts the two openers agree across 12 vectors,
  and TestOpenCustomEntryFile_NeverEscapes attacks the new path independently with 10 traversal/symlink
  vectors asserting content absence. Verified before the change that os.Root files are seekable (ServeContent
  requires io.ReadSeeker) and that Range yields a real 206.


  Also added an e2e test asserting a real 304 over HTTP, since the previous ticket showed Go tests alone
  can miss protocol behavior.'
---

Raised by `/code-review` of the TKT-IWMETE implementation.
