---
id: RR-2P1EBB
type: review-response
title: CodeMirror guard keys on a class name rather than editor identity
finding: |-
    [security] `isInputFocused()` (frontend/src/utils/dom.ts:12) returns true when `el.closest('.CodeMirror')` matches, i.e. it makes a trust decision on a class name rather than on editor identity. The two handlers changed for BUG-DNP5E7 become new consumers of that branch.

    The security reviewer raised this as `minor`, arguing entity content can forge the class and silently suppress the `/` and `f` shortcuts (a denial of affordance). VERIFIED NOT EXPLOITABLE, and downgraded to `nit` on that basis.

    The reviewer tested DOMPurify in isolation and found `<div class="CodeMirror"><a href="#" tabindex="0">click me</a></div>` survives "byte-identical". Running the same input through the actual `renderMarkdown()` pipeline shows it does not: the focusable `<a tabindex="0">` survives, but the wrapping `div.CodeMirror` is DROPPED. Sanitized output is `<a href="#" tabindex="0">click me</a>`.

    The attack needs BOTH halves on the same subtree — a surviving `.CodeMirror` ancestor AND a focusable descendant. Only the descendant survives, so `closest('.CodeMirror')` returns null and the guard cannot be forged. Confirmed by a test that renders the payload through `renderMarkdown`, focuses the surviving anchor, and asserts `isInputFocused()` is false.

    Also confirmed: the Milkdown/ProseMirror branch is not forgeable either, since DOMPurify strips `contenteditable`.

    Remaining substance is design taste, not security: a class-name check is a weaker coupling than asking the editor instance for its DOM root, and `isInputFocused()` has six call sites, so a future consumer using it for something more consequential than suppressing a navigation shortcut would inherit that coupling. Pre-existing (unchanged since the SPA migration in ebbcb5b6) and out of scope for this bug.
severity: nit
reason: |-
    Not exploitable, and pre-existing. The security reviewer's proof-of-concept was tested against DOMPurify in isolation; run through the actual `renderMarkdown()` pipeline, the payload `<div class="CodeMirror"><a href="#" tabindex="0">click me</a></div>` sanitizes to `<a href="#" tabindex="0">click me</a>` — the focusable anchor survives but the `.CodeMirror` wrapper is dropped. The attack needs both halves on the same subtree, so `closest('.CodeMirror')` finds nothing and the guard cannot be forged. Verified by a test that renders the payload, focuses the surviving anchor and asserts `isInputFocused()` is false.

    What remains is design taste, not security: keying on a class name is weaker coupling than asking the editor instance for its DOM root. `frontend/src/utils/dom.ts` is unchanged since the SPA migration (ebbcb5b6) and is not touched by this bug's diff, which only redirects two call sites to it. Hardening it — or adding `FORBID_ATTR: ['tabindex']` to renderMarkdown — is a separate change that should be judged on its own merits rather than folded into a shortcut-guard fix.
status: wont-fix
---

## Verification

Run through the real pipeline, not the sanitizer alone:

```
input:  <div class="CodeMirror"><a href="#" tabindex="0">click me</a></div>
output: <a href="#" tabindex="0">click me</a>
```

The `.CodeMirror` wrapper does not survive, so there is no ancestor for
`closest()` to find. The forged-guard path requires both the ancestor and the
focusable descendant; only one survives.

## Recommendation

No change for this bug. If hardened later, prefer anchoring on the editor
instance's own DOM root over a class selector. `FORBID_ATTR: ['tabindex']` in
`renderMarkdown` would remove the focusability prerequisite generally, but is
unrelated to this fix and should be judged on its own merits.
