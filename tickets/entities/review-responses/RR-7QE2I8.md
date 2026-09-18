---
id: RR-7QE2I8
type: review-response
title: Withdrawal left the write-back guard disarmed, and the comment justifying it was wrong
finding: 'Pressing the entity-reference button and dismissing it removed the `@` but left the document
  marked edited, so every later .value read returned a fully reserialized body (tables repadded, setext
  rewritten) for an entity nobody edited. The comment asserting this was unavoidable claimed the trailing
  plugin left a paragraph the delete could not remove. A wrong rationale in a load-bearing comment is
  worse than none: it is why the defect survived a review looking straight at the function.'
severity: significant
resolution: 'Restoration is now gated on the OBSERVABLE: the serialization after withdrawal is compared
  against the one captured immediately before the trigger was typed. Two subtleties the fix had to get
  right, each found by probing rather than reasoning. Comparing against _originalValue could never match,
  because a WYSIWYG round-trip reformats every body it opens, so the restore would have been dead code
  that looked like a fix. And both pre-insertion values must be captured BEFORE view.dispatch, which runs
  the dirty tracker synchronously — reading this._dirty after it always reported true. On the disputed
  claim itself: the review''s probe measured textContent and concluded the document fully reverts; a structural
  probe showed a leftover EMPTY paragraph (P,TABLE becoming P,TABLE,P) that textContent cannot see. So
  the original comment was closer to right than its disproof, and both were reasoning where a measurement
  was needed. The comment now says exactly that.'
status: addressed
---
