---
id: RR-CR2-AC11COMMENT
type: review-response
title: The AC11 test comment claimed two directory spellings hit different guards; they hit the same one
finding: 'The comment asserted ''fonts'' reaches the IsDir check while ''fonts/'' is rejected earlier
  by fs.ValidPath. VERIFIED FALSE by probe: path.Clean(''/fonts/'') yields ''/fonts'', so validCustomEntry
  returns (''fonts'', true) for BOTH spellings and both die at IsDir. fs.ValidPath never sees a trailing
  slash. The assertion still passed, so this was a comment defect rather than a behaviour defect - but
  the dangerous kind: it documents a guard that does not exist, so someone simplifying the IsDir check
  would believe fs.ValidPath still covers ''fonts/''.'
severity: minor
status: addressed
resolution: Comment corrected to state that both spellings normalise identically and die at the same IsDir
  check. Added a 'both spellings normalise identically' subtest asserting validCustomEntry returns the
  same result for each, so the real (single) guard is pinned rather than an imagined second one.
---

Raised by `/code-review` of the TKT-IWMETE implementation.
