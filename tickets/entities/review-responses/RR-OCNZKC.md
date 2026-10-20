---
id: RR-OCNZKC
type: review-response
title: The edit flow is not designed, and PATCH replaces all props, including refs the author can no longer read
finding: '§2 and §4 design only the post composer, yet §0 and AC 7 promise edit. `EditMessageRequest` says "Props replaces every property". A held message''s ref prop is a RefProp, and a restricted one has no id, so an edit form cannot send the ref back: it is either erased (optional prop) or rejected (required prop). This is the read-modify-write erasure that the project''s PatchEntity rule names. Fix: specify edit mode (the composer prefilled from the message, the same field mapping, sending ids for readable refs). Block edit (hide the button, or show it disabled with a reason) when any ref prop is restricted, unless the server gains a "keep" sentinel. Add a test that editing a message with a restricted ref sends no request that drops it.'
severity: significant
resolution: Edit mode prefills the composer from the message, and the server leaves edit out of _actions when the message holds a ref the caller cannot read, so no PATCH can erase it (R7, AC 11).
status: addressed
---
