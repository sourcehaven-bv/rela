---
id: RR-4GELDN
type: review-response
title: Withdrawal consumed its record before checking it could act
finding: '_withdrawPromptedRef nulled _promptedRefSpan unconditionally, then hit four early returns. The
  readonly case is ordinary and reachable — an app locking the editor on save — and permanent: press the
  reference button, app sets readonly, press Escape, and the span is discarded without the `@` being removed.
  Removing readonly and pressing Escape again does nothing. The stray character is stranded for the session,
  which is the exact failure RR-LF36QW was filed about, reintroduced through a different door. The existing
  ''withdraws only once'' test passed green over this because it exercises only the success path.'
severity: significant
resolution: 'The span is consumed only on a path that resolves it: the text was removed, or it no longer
  matches so it is the user''s now. A withdrawal that merely cannot run yet leaves the span in place.
  Pinned by a test that sets readonly, presses Escape, asserts the `@` survives, then clears readonly
  and asserts the second Escape still works.'
status: addressed
---
