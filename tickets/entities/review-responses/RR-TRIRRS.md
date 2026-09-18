---
id: RR-TRIRRS
type: review-response
title: Reference button destroyed the user's selection
finding: '_promptForRef dispatched insertText(prefix, from, to) using the live selection. insertText REPLACES
  that range, so with a non-empty selection the highlighted text was deleted: `hello world` with `world`
  selected became `hello  @`. Recoverable only by an undo the user has no reason to expect after pressing
  an *insert* button. Compounding it, the boundary character was read at `from`, which is inside the range
  about to be deleted, so the prefix decision was made against text that would not exist.'
severity: significant
resolution: 'Insert AT the selection end instead of over it: insertText(prefix, to, to), with the boundary
  character read at `to`. Verified the original defect first by driving a real ProseMirror selection (happy-dom
  cannot deliver one through the DOM), then pinned it with a test that selects a word and asserts it survives.'
status: addressed
---
