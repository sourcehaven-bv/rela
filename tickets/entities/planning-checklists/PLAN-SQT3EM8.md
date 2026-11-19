---
id: PLAN-SQT3EM8
type: planning-checklist
title: 'Planning: Link display for URL string properties'
started: "2026-11-18"
completed: "2026-11-18"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: a schema-level opt-in `display: {link: true}` on a single-valued
string property; a registered `url` widget that shows the value as a link in
display mode and as a text input with the URL keyboard in edit mode; routing
through the widget registry on detail pages, view sections, list cells and
kanban cards; docs. Out: link text other than the URL itself; a Configure UI
editor for the new key; URL validation on save.

**Acceptance Criteria:**
1. An `https:` value shows as a link with `target="_blank"` and `rel="noopener noreferrer"` on the detail page, a list cell and a kanban card (e2e `url-link.spec.ts`).
2. A `javascript:`, `data:` or relative value shows as plain text (unit `linkHref.test.ts`, e2e).
3. The edit form shows a text input with `inputmode="url"`, and a stored non-URL value does not block a save (e2e).
4. The schema loader rejects `link` on a non-string, list or `values:` property, and `currency` together with `link` (Go `display_test.go`).

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: small change on an existing mechanism)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- `display: {currency: ...}` in `internal/metamodel` is the existing schema-level display opt-in; `link` is a second key on the same map.
- `frontend/src/components/forms/milkdown/linkUrl.ts` already holds an http/https/mailto allowlist for markdown links; the new `linkHref` helper shares that list.
- The widget registry (`defaultWidgetFor`, `hintKindToWidgetName`, `densePropertyRoutingHint`) routes the currency widget the same way.
- Browser `URL` parsing normalises case and strips leading whitespace and tabs, the same way the browser reads an `href`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Add `Link bool` to `metamodel.PropertyDisplay` and the
API wire type, validated in the loader. Add `WidgetURL = "url"` to the Go widget
table. In the SPA, add `UrlWidget.vue`, register it for `string`, and route
`display.link` to it in `defaultWidgetFor`, `densePropertyRoutingHint` and
`viewFieldRoutingHint`. The href comes from `linkHref`, which parses with `new
URL` and allows only http, https and mailto.

Alternatives: a `format: url` key does not exist on strings, and adding one next
to `display:` would split display options over two keys. A per-field `widget:
url` override applies only to forms and inline-edit sections, so it cannot reach
list cells or kanban cards.

**Files to modify:** internal/metamodel/{types,loader}.go,
internal/apiwire/v1/responses.go, internal/dataentry/{api_v1,helpers,config}.go,
internal/dataentryconfig/{config,validate}.go, internal/configedit/allowlist.go,
frontend/src/widgets/{UrlWidget.vue,registry.ts,types.ts,viewRouting.ts},
frontend/src/utils/linkHref.ts, frontend/src/components/entity/EntityDetail.vue,
frontend/src/views/KanbanView.vue, docs guides, e2e fixtures and spec.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** The property value is user-entered entity data
and may hold any string. On read, `linkHref` allowlists the scheme after browser
URL parsing; anything else renders as text. A mailto link drops its query and
fragment. The schema key is operator config and is validated at load.

**Security-Sensitive Operations:** Rendering a user-supplied href. Protected by
the scheme allowlist, Vue attribute binding (no v-html), `rel="noopener
noreferrer"` and `target="_blank"` for web links. Field-level `visible:`
redaction happens server-side before the value reaches the widget.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
1. Unit: `UrlWidget.test.ts` for link attributes, mailto, click.stop and edit input; e2e for detail, list, kanban and form.
2. Unit: `linkHref.test.ts` scheme allowlist; e2e javascript: value.
3. e2e form save with a hostile value.
4. Go loader tests and `TestToV1PropertyDef_Display`, `resolveWidget` cases.

**Edge Cases:** Mixed-case scheme, leading whitespace, embedded tab and newline,
entity-encoded colon, zero-width and BOM prefixes, protocol-relative and
relative values, bare host, empty value, non-string value, list value.

**Negative Tests:** javascript:, data:, vbscript:, ftp:, file: render as text;
the loader rejects link on int, list and enum properties and with currency.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** A link nested in a clickable card or row could trigger both the link
and the card. Mitigated with `@click.stop` and a kanban click guard. Effort: s.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/metamodel.md - New metamodel features
- [x] docs/data-entry.md - UI changes

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: small change on the existing display: mechanism; the code review and security review covered the design)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** N/A
