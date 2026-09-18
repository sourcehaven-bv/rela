---
id: RR-3IASH2
type: review-response
title: Source scan missed 9 of 11 realistic hand-rolled guard variants
finding: |-
    The scan's first assertion matched only the two literal spellings it was written against. The reviewer constructed 11 realistic hand-rolled guards; 9 slipped past, including three that are not exotic at all: an array literal split across lines (what Prettier does to a long line), a hoisted `const TAGS = [...]` (what anyone does when they need the list twice), and `tagName` appearing on the line after the literal.

    The structural flaw was the line-by-line loop: a guard formatted across two lines is invisible by construction. The aliasing gap was equally real — `localName`, `nodeName`, `tagName.toLowerCase()`, `instanceof HTMLInputElement` and `matches`/`closest` all express the same decision and none matched.

    The reviewer's headline claim needs one correction. They reported that a `localName` mutation left 'all 3 assertions green'. Re-running it, the mutation IS caught — by assertion 3, because replacing the guard also drops the now-unused import. The genuine bypass is narrower and sharper: retain the import but bypass the call. That variant did pass, and it is the case that actually mattered.
severity: significant
resolution: |-
    Rewrote the scan.

    1. Whole-file matching instead of line-by-line, with comments stripped first, so multi-line and hoisted guards are visible.
    2. Widened to the aliases that mean the same thing: `tagName|nodeName|localName` (case-insensitive tags), `instanceof HTMLInput/TextArea/SelectElement`, and `matches`/`closest` against a bare tag selector.
    3. The selector pattern uses a `(?<![.#\w-])` lookbehind so a CSS CLASS containing the word is not swept up — caught a real false positive on `.closest('.entity-target-select')` in EntityTargetSelect.vue.

    Re-ran the reviewer's matrix: 10/10 variants now caught, up from 2/11. Also caught with the import retained — the sharper case the reviewer's own test missed.

    Kept assertion 1 rather than deleting it as the reviewer suggested. With whole-file matching it now catches the variants that motivated deletion, and it fails at the offending line rather than only at the missing-import level, which is the more useful diagnostic. The docstring records that an ESLint `no-restricted-syntax` AST selector is the immune long-term answer, as follow-up rather than blocking.
status: addressed
---

Finding 2 from the cranky-code-reviewer.

Bypass matrix after the rewrite (import removed — guard fully replaced):

| Variant | Before | After |
|---|---|---|
| `['INPUT','TEXTAREA'].includes(tagName)` | caught | caught |
| `tagName.toLowerCase()` | missed | caught |
| `localName` | missed | caught |
| `nodeName` | missed | caught |
| `matches('input, textarea')` | missed | caught |
| `instanceof HTMLInputElement` | missed | caught |
| `closest('input,textarea')` | missed | caught |
| multi-line array literal | missed | caught |
| hoisted `const TAGS` | missed | caught |
| no guard at all | caught | caught |
