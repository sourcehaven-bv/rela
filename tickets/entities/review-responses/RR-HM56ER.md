---
id: RR-HM56ER
type: review-response
title: Reuse of the existing comment components is not considered
finding: '§2 builds the message list and composer chrome from scratch, while rela-components already has RlCommentThread (row with edit-in-place, author and time), RlCommentComposerBox and RlCommentsPanel. Fix: in T7a research, check whether row chrome, the composer box and the edit-in-place state can be shared or extracted, and record why not if they cannot. A second visual language for "a message by a person" next to the comments panel is the drift the library exists to stop.'
severity: minor
resolution: T7b starts with a check of RlCommentThread, RlCommentComposerBox and edit-in-place for reuse and records the outcome in the PR and checklist (R22).
status: addressed
---
