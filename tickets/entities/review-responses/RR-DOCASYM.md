---
id: RR-DOCASYM
type: review-response
title: The read/write asymmetry comment asserts a dichotomy the document path breaks
finding: 'The asymmetry (nil reader -> methods present-and-raising; nil mutator -> methods absent) is
  justified at runtime.go:1942-1952 as: nil reader = misconfiguration worth naming; nil mutator = deliberate
  posture.


  Coherent for the cascade path, where luascriptrunner.go sets reader and manager together under one ElevatedProvider
  check, so a nil reader really is a wiring bug. Shakier now: documentService.elevatedDeps sets ElevatedReader
  from s.elevation(), and document.go:157-160 explicitly contemplates s.elevation == nil as legitimate
  -- "Nil when the wiring site granted none, in which case an elevated document renders WITHOUT bypass_acl
  rather than failing".


  So a nil reader is now ALSO a deliberate posture on one path, producing a different failure shape (binding
  absent) than the raise was designed for (binding present, method raises). Both fail closed, so the behavior
  is fine; the comment asserts a clean dichotomy that no longer holds. Tighten it or note the third state.


  Confirmed NOT regressed: closure scoping and self-invalidation still hold on the reader-only path --
  live flips in the same defer regardless of which handles were wired, recordElevatedReads rides that
  defer so a raising closure still audits, and readGuard composes both checks.'
resolution: |
  Tightened the comment. It now states that the misconfiguration-vs-posture dichotomy fits the cascade path (where reader and mutator are wired together under one check) and calls out the document path's third state: a wiring site may grant no elevation bundle at all, so neither handle is set and bypass_acl is absent outright rather than present-and-raising.
  
  All three states fail closed; the comment now says they fail in different shapes rather than implying a clean two-way split.
  
  The reviewer separately confirmed no regression in the TKT-D8T148/ACSBSA guarantees on the reader-only path (live flips in the same defer, recordElevatedReads rides it so a raising closure still audits, readGuard composes both checks).
severity: minor
status: addressed
---
