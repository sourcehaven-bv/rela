---
id: PLAN-P61Q5X
type: planning-checklist
title: 'Planning: Escape feed text in entity bodies'
started: "2026-10-14"
completed: "2026-10-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope: escaping subscription text values in the two markdown-body sinks of a
mapping (`create_if_missing.content`, `then.append_section.content`); making
`item.link` and `feed.link` safe in a body; the `ingest.Source` contract change;
docs.

Out of scope: webhook values (they stay verbatim, because a producer may send
markdown on purpose and sits behind the authenticating proxy); properties
(stored values, not markdown); Lua writes.

**Acceptance Criteria:**
1. A feed title `![x](https://t/p.gif)` in create content is stored escaped and renders as literal text. Test: ingest unit test plus a subscription run test asserting the stored body.
2. The same title in a property is stored unchanged. Test: same run test.
3. Webhook values in a body are unchanged. Test: existing dataentry webhook tests pass; a new ingest test for `ValueRaw` in append_section.
4. A link with `![x](...)` in its query reaches the body without raw `[`, `]`, `(`, `)`, `<`, `>` or `!`. Test: source_test link case.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small change)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: CommonMark spec section 2.4 defines backslash escapes)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- `transform.EscapeInline` (internal/transform/entity_renderer.go:230) escapes only `|` and `\` for table cells; it does not stop images, links or HTML.
- `lua/markdown.go:2589 escapeMarkdownLinkText` escapes link text only.
- CommonMark allows a backslash before any ASCII punctuation, so over-escaping is safe; a `strings.Replacer` is single-pass.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
- `ingest.ValueKind` with `ValueRaw`, `ValueText`, `ValueMarkdown`; `Source.IsBlock` becomes `Source.Kind(ref) ValueKind`.
- `interpolateWith(tmpl, src, sink)` where sink is property or body (create content keeps markdown lines; append content flattens them). In a body sink, `ValueText` values pass through `EscapeMarkdown` after flattening.
- `EscapeMarkdown` backslash-escapes ``\ ` * _ [ ] < > & ! # | ~``.
- subscription `itemSource.Kind`: summary and content are markdown; title, id, authors, categories and feed.title are text; the rest are raw. `link()` percent-encodes `[ ] ( ) < > ! * _ \` ` ` ` in the raw query and fragment.
- webhookPayload.Kind returns `ValueRaw `.

Alternatives: refusing text refs in content (rejected: `{{item.title}} ` in a
body is the natural mapping); escaping webhook values too (rejected: breaks
producers that send markdown on purpose, and their sender is trusted).

**Files to modify:** internal/ingest/{ingest.go,interpolate.go,ingest_test.go},
internal/subscription/{source.go,source_test.go,run_test.go,config.go},
internal/dataentry/webhook_routes.go, docs/feed-subscriptions.md.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** feed fields (untrusted). Text kinds are escaped
at the body sink; markdown kinds come only from htmlmd's sanitizer; raw kinds
are URLs rela built (scheme allowlist, percent-encoded punctuation), timestamps
or operator names. Each subscription ref has a declared kind; an unknown ref is
already refused at config load.

**Security-Sensitive Operations:** writing markdown that the SPA renders through
marked and DOMPurify. DOMPurify keeps `<img> `, so escaping is the control
against tracking images.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see the acceptance criteria; the appbuild e2e feed gains a
title with image markup, and the test asserts the stored body.

**Edge Cases:** an empty value; a value that is only punctuation; an existing
backslash (escaped once, never re-scanned); newlines (flattened before
escaping); NUL; markdown in append content (flattened, not escaped).

**Negative Tests:** escaped markup must not parse as an image (check the escaped
string has no unescaped `![ `).
