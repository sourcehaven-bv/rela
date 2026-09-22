---
id: RR-MYTU5V
type: review-response
title: handlePaste swallowed pastes it could not apply, losing the clipboard
finding: 'linkPaste returned true on three conditions it had not verified, so ProseMirror''s own paste handling never ran. (1) A selection spanning two blocks produced one link PER BLOCK from a single pasted URL. (2) canApplyLink was an any-node test, so a selection reaching into a code block passed it and the paste was swallowed and mangled. (3) A selection partly overlapping a link fell through to the default paste, which destroyed the link: ''see [docs](url) here'' became ''shttps\://new\.test/[s](url) here''.'
severity: critical
resolution: canApplyLink is now an all-nodes test, a new isWithinOneBlock guard refuses cross-block selections, and pasteWouldDamageLink makes the plugin CLAIM a partial-overlap paste and do nothing rather than decline it (declining is what let the default handler shred the link). Each guard has a unit test, and each was mutation-verified by reverting it and confirming the test fails.
status: addressed
---

**Finding (code review).** `handlePaste` returning `true` tells ProseMirror the
paste is handled. The plugin returned `true` in three situations where it then
could not do what it had claimed, so the user's clipboard went nowhere.

All three were verified against the real editor before fixing:

1. **Multi-block.** Selection 2..10 across two paragraphs, pasting
`https://new.test/`, gave
`a[lpha](https://new.test/)\n\n[be](https://new.test/)ta` — one link per block,
from one pasted URL.
2. **Code block.** `canApplyLink` walked `nodesBetween` and returned true if
ANY text node accepted the mark. A code block declares `marks: ''`, so the mark
could not apply and the paste was lost.
3. **Overlapping a link.** Refusing to linkify was not enough: falling through
handed the selection to the default paste, which replaced it. `see
[docs](https://old/) here` came back as `shttps\://new\.test/[s](https://old/)
here` — the word gone, the link reduced to one character, the URL left as
escaped literal text.

The module's own header warned about exactly this ("Getting this wrong does not
produce a wrong link; it produces a paste that silently does nothing") and its
comments asserted guarantees no test checked.

**Resolution.**

- `canApplyLink` is now an ALL-nodes test. An "any" test reads as the
permissive-sounding choice and is the dangerous one.
- `isWithinOneBlock` refuses a selection crossing a block boundary.
- `pasteWouldDamageLink` makes the plugin **claim** a partial-overlap paste and
do nothing. Declining is what allowed the damage; a selection wholly inside a
link is still an ordinary label edit and falls through as before.
- The raw-clipboard whitespace check moved BEFORE `normalizeLinkUrl`, which
strips whitespace and so made the later check unreachable.

Each guard has a unit test, and each test was mutation-verified: reverting the
guard makes it fail. The code-block test asserts the paste still *happens* as
plain text, rather than the vacuous "no link was created" — which passed against
the bug.
