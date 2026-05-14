---
id: PLAN-BURP
type: planning-checklist
title: 'Planning: Markdown renderer preserves source line breaks in data-entry view'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN:
- Change `breaks: true` → `breaks: false` (or remove the option) in
`frontend/src/utils/markdown.ts`, the single call site for `marked.parse` in the
data-entry SPA.
- Update or add unit tests in `frontend/src/utils/markdown.test.ts`
asserting the new behavior (single newlines do NOT become `<br>`;
two-trailing-spaces still do).

OUT:
- Server-side rendering paths (goldmark in `internal/lua/markdown.go`
and friends). Already CommonMark — not affected.
- Markdown editor input behavior — editor stores source verbatim;
this is purely a render-side change.
- Reflowing existing entity files. Source stays wrapped at ~80; the
point is the render no longer mirrors that wrap.

**Acceptance Criteria:**

1. A paragraph wrapped across multiple source lines renders as one
continuous paragraph in HTML. Test: input `"foo\nbar"` → assert no `<br>` tag in
output; both words inside a single `<p>`.
2. A paragraph with two trailing spaces still produces a hard break.
Test: input `"foo  \nbar"` → assert one `<br>` tag inside the `<p>`.
3. Lists, headings, code blocks, blockquotes are unaffected. Test: the
existing tests for these constructs continue to pass without edits.
4. Manual verification in browser: load an entity with a wrapped
description (e.g. a ticket whose description spans 4–5 source lines). Confirm
the description reflows on viewport resize and no `<br>`s appear between the
wrapped lines.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- marked.js already has a `breaks` option (default: `false`). The fix
is removing the explicit `true`. Nothing to import, nothing to reinvent.
- Server-side rendering (`internal/lua/markdown.go`, goldmark via
`FEAT-010`) does NOT use HardWrap. CommonMark default. Verified via grep for
`hardwrap`/`HardWrap`. The frontend was the outlier.
- GitHub renders comments with `breaks: true` (that's where the option
comes from); rendering long-form markdown documents uses CommonMark default. Our
entity content is closer to long-form documents.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Remove the `breaks: true` option from the `marked.parse` call in
`frontend/src/utils/markdown.ts`. Single character/line change. GFM stays on
(tables, task lists). The `walkTokens` and entity-ref-rewrite path are
untouched.

**Alternatives considered:**
- *Keep `breaks: true`, fix the symptom with CSS* (e.g. `br { display:
none }`). Rejected: clobbers legitimate `<br>` from explicit hard breaks; user
can't author a hard break at all.
- *Pre-process the markdown source to collapse single newlines into
spaces before passing to marked.* Rejected: re-invents CommonMark's soft-break
rule and breaks code-block content.
- *Switch markdown library*. Rejected: marked.js is already wired;
scope way beyond this fix.

**Files to modify:**

- `frontend/src/utils/markdown.ts` — flip the option.
- `frontend/src/utils/markdown.test.ts` — add 2 regression tests
(soft break, hard break). Audit existing tests for any that may have implicitly
depended on `breaks: true` behavior.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- Input is entity content from the store, already rendered through
`marked.parse` and sanitized by `DOMPurify.sanitize` (lines 62-64). Changing
`breaks` does not change the sanitization path — output is still purified.

**Security-Sensitive Operations:**

- None. The change is a render-config tweak; the XSS-defense
surface (DOMPurify) is unchanged.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- AC1 (soft break → space): `renderMarkdown('foo\nbar')` → expect
rendered HTML contains `foo` and `bar` inside a single `<p>`, with no `<br>`
between them.
- AC2 (hard break preserved): `renderMarkdown('foo  \nbar')` → expect
exactly one `<br>` in the output.
- AC3 (lists/headings/code unaffected): existing tests cover these;
rerun the full file to confirm.
- AC4 (manual): `npm run dev`, open an entity whose source is
hard-wrapped; resize window; check no `<br>` between wrapped lines.

**Edge Cases:**

- Empty content: already handled (`if (!content) return ''`). Unchanged.
- Code blocks with newlines: stay as `<pre><code>…</code></pre>`;
marked treats code blocks independently of the `breaks` flag.
- Lists where each item is on its own line: still one `<li>` per item.
`breaks` only affects inline soft breaks inside paragraphs and similar inline
contexts.
- Tables (GFM): `breaks` does not affect GFM table parsing.

**Negative Tests:**

- N/A — the change does not introduce a new input-validation surface.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs)

**Risks:**

- *User astonishment for anyone who deliberately relied on
one-newline-per-line behavior.* Mitigation: this is consistent with every
general-purpose markdown renderer (CommonMark, goldmark, pandoc); the data-entry
server already uses CommonMark elsewhere. Users who genuinely want a line break
can either add two trailing spaces or insert a blank line. The CHANGELOG/PR
description will call this out.
- *Test breakage if any existing test snapshot encodes a `<br>`.*
Mitigation: audit `markdown.test.ts` before flipping and update.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] N/A - Behavior change is internal to the renderer; no command,
API, or config surface changes. CommonMark soft-break semantics are the de-facto
standard and don't need to be documented per-project.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: xs
enhancement with one-line render-config flip; no architectural decisions to
review)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A:
no design review performed)

**Design Review Findings:** None
