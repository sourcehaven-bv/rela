---
id: RR-0FRDLA
type: review-response
title: Pending trigger span survived into a different document
finding: _promptedRefSpan records positions in the document that produced it, but was not cleared in _unmount
  or in the value setter — the two places every other document-scoped flag is reset. Confirmed it survives
  a full programmatic replacement, still pointing at the old offsets. Only the ` ?@` text check stood
  between that and silently eating two characters from a new body that happened to have ` @` at the same
  offset, which is defence-in-depth doing load-bearing work by luck.
severity: minor
resolution: Cleared in both places, alongside _dirty/_armed/_activeMatchLength/_dismissedQuery. Pinned
  by a test that opens a prompt, loads a different body containing ` @`, and asserts Escape leaves it
  untouched.
status: addressed
---
