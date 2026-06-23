---
id: PLAN-TCHUKN
type: planning-checklist
title: 'Planning: Attachment web read path: ACL-gated download endpoint + file widget/preview'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (below)
- [x] Acceptance criteria documented with test scenarios

**Scope:** IN: (1) GET attachment-bytes endpoint, ACL-gated, entity-first
resolution; (2) `_attachments` metadata on per-entity payloads so the SPA knows
which file properties have a file; (3) a read-capable file widget replacing
TextWidget for `file` properties (filename, size, download link, inline image
preview). OUT: upload/delete (TKT-RXFD5B), multi-file `max` (TKT-WLLRO7), MCP
tool (TKT-R6U15C), the deeper fsstore rename re-stamp fix (the endpoint resolves
by current entity ID so it is *correct* regardless; the stale frontmatter string
is a separate store bug).

**Acceptance Criteria:**
1. Allowed user GETs an entity's attachment → 200 + bytes + headers.
2. Denied user → 404 (NOT 403), byte-identical to nonexistent, no ETag (RR-NGMI).
3. Missing attachment / missing entity / wrong property → 404.
4. Renamed entity: attachment reachable by current id, gone from old id.
5. SPA shows filename/size + download + image preview; non-image → download; edit mode read-only.

## Research

- [x] Checked codebase for similar patterns
- [x] Reviewed prior art

**Existing Solutions / prior art:** Route dispatch `handleV1DynamicRoutes`
(api_v1.go:279), `_actions` case mirrored for `_attachments`. ACL gate
`gateReadOrNotFound` (api_v1.go:794) reused verbatim. Serve-bytes precedent:
theme logo (nosniff+CSP) / theme export
(Content-Disposition+`safeThemeFilename`). Closed-world DTO
`_fields`/`_relations`. Store `AttachmentManager` (ReadAttachment→io.ReadCloser,
ErrNotFound→404). Test harness `acl_get_test.go`. Frontend registry +
`WidgetProps` + `Entity._fields?`.

**Research Doc:** N/A (small, well-patterned change)

## Approach

- [x] Approach chosen, builds on existing patterns, alternatives considered, deps identified

**Technical Approach:** Sub-resource `case "_attachments"` in route dispatch →
`handleV1GetAttachment` (gate→load→validate file-property→ReadAttachment→stream
with defensive headers). `V1Attachment` DTO + `_attachments` on `V1Entity`,
populated in `serializeEntityForWire` (every per-entity response, like the
affordance maps). FileWidget.vue (display + image preview; read-only note in
edit mode), registered for `file`; `_attachments` threaded via
`PropertyItem`→`PropertyDisplay`→widget, populated in EntityDetail's
`mapFieldsToProperties`.

**Files modified:** handlers_attachment.go (new), api_v1.go, affordances.go,
handlers_attachment_test.go (new), router_walk_test.go, api_v1_test.go
(fixture); FileWidget.vue (new), registry.ts, widgets/types.ts, types/entity.ts,
PropertyDisplay.vue, EntityDetail.vue, widgets.test.ts, registry.test.ts;
docs/data-entry/api-reference.md.

**Alternatives rejected:** dedicated system route (duplicates plural→type
resolution); `http.ServeContent` Range support (no precedent; defer).

## Security Considerations

- [x] Input sources, validation, sensitive ops, error-leak handling identified

**Input Sources & Validation:** entityID/property/plural from URL. plural→type
via resolver (unknown→404). property validated against declared `file`
properties (allowlist→404). Bytes resolved from `(entityID, property)` only —
never the stored path string; no caller-supplied path reaches the FS (no `../`
surface).

**Security-Sensitive Operations:** gate before store touch (no existence
side-channel; deny=404=nonexistent, body matches gate via shared
`entityNotFoundTitle`). User bytes served with nosniff + CSP `sandbox;
default-src 'none'` + `Content-Disposition: inline` + sanitized filename
(stored-XSS + header-injection guard). Store/gate errors mapped, logged
server-side, never leaked.

## Test Plan

- [x] Scenarios per criterion, edge cases, negatives, integration approach

**Test Scenarios:** allowed→bytes+headers; denied→404 parity+no ETag+matches
gate body; missing/unknown/non-file→404; rename→new-id 200/old-id 404; deceptive
.png→image/png+nosniff; .svg→sandbox CSP+inline; `_attachments` on per-entity
GET+mutation, absent on list rows; FileWidget edit-mode read-only. **Edge
Cases:** empty/missing property; filename with quotes/`..`/unicode (sanitized);
image vs non-image; hidden-by-ACL. **Negatives:** gate error→504/500 no leak;
property with `/` rejected upstream.

## Risk Assessment

- [x] Technical + security risks assessed; effort estimated

**Risks:** ListAttachments ContentType empty on fsstore → derive from filename
(mitigated). Stale frontmatter post-rename → endpoint resolves by current id
(correct); underlying re-stamp a follow-up. Inline malicious bytes → nosniff+CSP
sandbox (mitigated, now tested). Effort: m (confirmed).

## Documentation Planning

- [x] User-facing docs identified

**Documentation Impact:**
- [x] docs/data-entry/api-reference.md — new `_attachments` endpoint + payload. Docs-checklist DOCS-KU30G9 created at review.
- [x] N/A for metamodel/CLI (no schema or command change).

## Design Review

- [x] ~~Run `/design-review` before implementation~~ (N/A: small single-pattern change — mirrors `_actions` + theme-serve + closed-world DTO — and the security-sensitive ACL invariant is pinned by the existing test contract reused verbatim. The cranky `/code-review` at review stage covered it and surfaced 4 findings, all addressed.)
- [x] All critical/significant findings addressed in plan — surfaced at code-review instead: RR-FKJYMC/SQE8VA/XB5738/601TJD all addressed.

**Design Review Findings:** deferred to code-review (RR-FKJYMC, RR-SQE8VA,
RR-XB5738, RR-601TJD).
