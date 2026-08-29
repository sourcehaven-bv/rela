---
id: RR-KZLSV1
type: review-response
title: mdHelpers/mdASTConverter ls comment overstated LState identity; two leftover receiver shadows
finding: 'The new ls field comment claimed it ''is the same LState the bindings are registered on'' — false under coroutines: flow.go invokes bindings on a thread LState while ls stays Runtime.L. Safe only because gopher-lua''s NewTable ignores its receiver; a future receiver-sensitive call (Push/RaiseError/CheckX) on ls would act on the wrong state inside a flow. Additionally two loop variables still shadowed the receiver c (markdown.go CodeSpan case in appendInlines, and extractListItems), inconsistent with the two renamed during the extraction.'
severity: minor
resolution: 'Rewrote both ls field comments to state the real constraint (held ONLY for NewTable, which ignores its receiver; receiver-sensitive calls must use the invoking LState parameter). Renamed the two remaining shadowed loop variables (child/blk). Verified: build, lua tests, comment-lint.'
status: addressed
---

Review finding from the TKT-4WBLG6 code review (cranky-code-reviewer). Comment
accuracy on new prose plus consistency of the receiver-shadow renames; no
behavior impact.
