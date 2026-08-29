---
id: PLAN-XMWT23
type: planning-checklist
title: 'Planning: Declarative mails: content templates, scheduler + automation triggers, graph-resolved recipients with per-recipient ACL scoping'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**
Per-recipient scheduled mail only: declarative content templates, bounded
`for_each` expansion, one durable delivery job and one ACL-scoped render per
recipient, table/list/detail sections, and store-aware address validation.
Automation triggers are TKT-LU4AAY; scheduler fan-out is TKT-XWZIOB; the shared
job seam is TKT-YOED3R; HTTP/script transports are TKT-DS1CR6. No broadcast,
editor, preferences,
unsubscribe flow, new filter language, or preview endpoint.

**Acceptance Criteria:**
1. Parse a two-section template, run its scheduled task against a fixture store,
   and assert one message per selected recipient with table/list content and absolute links.
2. Render a detail section and assert its entity markdown appears in both body parts.
3. Table-test unknown entity types and malformed filters at config load.
4. Reference a missing template from a task and assert a named load error.
5. Run store-aware validation with missing/blank/ambiguous address properties and
   assert every offending entity and property is named; at runtime assert bad
   recipients are logged and valid recipients still enqueue.
6. Resolve a filtered recipient set and assert exactly one child job per recipient.
7. Deny one row and redact one field from a recipient principal; assert both are absent.
8. Match zero rows and assert a valid empty section in HTML and text.
9. Fail one delivery and assert only its child retries while successful peers do not.
10. Submit concurrent expansion work and assert its stable pending identity
    collapses duplicate child jobs; document the post-completion replay window.
11. Run `rela validate` fixtures for missing templates/types and malformed filters.
12. Table-test unknown keys at all new YAML levels, including typo suggestions.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: PLAN-EQC0Q8 already
  contains the feature-wide SMTP/rendering research; this slice adds no library)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** Pending design review; the feature-wide SMTP/rendering research is
captured in PLAN-EQC0Q8. This layer introduces no new third-party library.

**Existing Solutions:**
- `internal/dataentry/feed_provider.go` already parses section filters and queries
  entity lists; reuse its vocabulary and extraction patterns without coupling mail
  to the HTTP/feed surface.
- `internal/dataentryconfig.ListColumn` is the established property/relation column
  model. Reuse it rather than define a mail-only column dialect.
- `internal/mailrender` is the pure, sanitized model-to-HTML/text boundary;
  declarative mail assembles `mailrender.Message` and never emits HTML itself.
- PR #1444's `internal/jobs` provides the process-scoped asynchronous seam,
  memory/PostgreSQL durability tiers, handler registration, and retry policy.
  Scheduled mail must not add a second queue through `internal/mail.Outbox`.
- The memory sender remains the integration-test probe; no SMTP fake is needed here.
- `internal/appbuild.ScheduledLuaWriteDeps` and `internal/visibility` establish the
  ACL-bound reader wiring pattern. `run_as` remains identity, not capability
  (DEC-O59WM4).
- Scheduler fan-out is TKT-XWZIOB; field redaction already shipped in
  TKT-BUYEW1. The legacy scheduler ladder work in TKT-N52HRC is independent
  after TKT-YOED3R supplied bounded child-job retries. This ticket must not partially
  reproduce them.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
1. Add a leaf `internal/mailtemplate` package containing strict YAML-facing template,
   recipient, section, and style models plus syntactic validation against metamodel
   and `filter.ParseAll`. It assembles `mailrender.Message` from an injected reader;
   it does not know SMTP, scheduler state, or appbuild.
2. Load `mail_templates:` alongside the existing data-entry configuration so column
   semantics and metamodel validation share one source of truth. Unknown keys are
   rejected recursively with the existing suggestion convention.
3. Extend scheduler tasks to a closed `script` xor `template` action. Validate the
   referenced template set at application assembly/`rela validate`, where both
   configs are available; keep `scheduler.Config.validate` store-free.
4. Reuse the scheduler expansion and child job kinds from TKT-XWZIOB. A template
   is an action executed by the existing recipient child; do not introduce a
   second mail fan-out or delivery job layer.
5. The child resolves the recipient principal and current address, builds a row- and
   field-visible reader at the appbuild boundary, queries sections, renders once,
   stamps `mail.Message.RenderedFor`, and calls `mail.Sender` directly. Do not route
   a job through the foundation's in-memory outbox (double queue).
6. Use task + occurrence + recipient as the pending child identity. Concurrent
   copies collapse; the post-completion replay window remains explicitly at-least-once.
7. Add store-aware validation for recipient properties and matching entity values to
   the `rela validate` pipeline. Runtime repeats address validation because data can
   change after validation; one bad entity is logged/skipped without aborting peers.

Rejected: putting graph queries in `mailrender` (breaks its pure leaf boundary),
putting templates in `.rela/mail.yaml` (mixes operator-local transport secrets with
project content), broadcast rendering (cannot prove every recipient may read shared
content), and scheduler job -> mail outbox double queueing.

**Files to modify:**
- `internal/mailtemplate/{config,render,validate}.go` and tests (new)
- `internal/dataentryconfig/config.go`, `validate.go`, and tests
- `internal/scheduler/config.go`, `scheduler.go`, and tests
- `internal/appbuild/appbuild.go`, `mail.go`, and integration tests
- `internal/cli/validate.go` and validation tests
- `.go-arch-lint.yml`, `.testcoverage.yml`
- `docs/mail.md`, `docs/scheduled-tasks.md`, and the docs-project mail guide

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- Project YAML: strict known-key decoding; allowlisted styles; known metamodel types,
  properties, relations, columns, and parseable filters. Invalid config fails load.
- Entity properties/bodies: untrusted graph content; read only through the injected
  visibility wrapper and sanitized by `mailrender`.
- Recipient addresses: require one scalar, trimmed, non-empty, header-safe address;
  store-aware validation reports all offenders, runtime logs entity IDs and skips.
- Links/base URL: retain `mailrender`'s http/https allowlist and safe relative-link
  resolution. Template names are exact map keys, never paths.

**Security-Sensitive Operations:**
- Inbox export is irreversible: `RenderedFor` is mandatory and all reads originate
  from the ACL-bound dependency assembled for the resolved recipient; no shared
  `run_as` or raw store escape hatch.
- Recipient fan-out is bounded before enqueue to prevent memory/SMTP amplification.
- Logs name template/entity/property but never body, address values, credentials, or
  rendered message bytes.
- Field-level redaction reuses the completed TKT-BUYEW1 scheduled-read seam.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
The Understanding section maps each criterion. Unit tests cover strict parsing,
model assembly, styles, filter/column validation, address extraction, bounds,
pending identities, and empty results. Scheduler tests cover the script/template
union, expansion, child independence, and task liveness.
An appbuild integration test uses a real filesystem store, ACL policy, visibility
wrapper, renderer, job queue, and memory sender end-to-end.

**Edge Cases:**
- Missing/empty template maps remain backward compatible; template and task names
  must be non-empty and unique by YAML map semantics.
- Empty recipient/section results produce no sends/a valid empty section respectively.
- Missing, list-valued, whitespace-only, malformed, CR/LF, and duplicate addresses are
  rejected or skipped with deterministic diagnostics.
- Unicode content remains intact; hostile markdown/URLs are sanitized by mailrender.
- Exactly-at-limit recipients send; limit+1 logs the dropped count without silently
  truncating. Outbox-full is surfaced and logged by the producer.
- Config/store changes between validation and execution are handled by runtime checks.

**Negative Tests:**
Unknown keys/types/properties/templates/styles, invalid filters/columns/base URLs, a
task specifying both/neither script and template, and invalid recipient definitions
fail load/validate with location and name. A bad runtime recipient does not fail valid
peers; renderer or enqueue failure fails that scheduled task so existing scheduler
failure policy applies, while later tasks remain live.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- ACL exfiltration: remove broadcast; type the reader dependency, assemble it only
  through recipient visibility, require/stamp `RenderedFor`, and prove row and field
  denial end-to-end.
- Configuration ownership drift: keep content project-owned and transport local; add
  cross-config validation at assembly/validate rather than merging files.
- Recipient amplification: hard expansion bound and explicit log/counter.
- Duplicate delivery: stable pending identities suppress concurrent work; document
  post-completion expansion replay and SMTP acknowledgement crash windows rather
  than claiming exactly-once delivery.
- Scheduler regression: model task action as xor while preserving legacy script state
  keys and execution semantics; regression-test script-only configs.
- Dependency sequencing: TKT-U2R7GU depends on TKT-XWZIOB and the completed
  TKT-YOED3R job seam. TKT-N52HRC is no longer a blocker. Do not enter implementation until the design review
  confirms the recipient-scoped action boundary.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] `docs/mail.md` — template schema, recipient ACL model, recipient validation,
  bounds, failure behavior, and field redaction
- [x] `docs/scheduled-tasks.md` — `template` action and `run_as` semantics
- [x] docs-project mail guide — complete operator example and troubleshooting
- [x] `docs/cli-reference.md` — new `rela validate` diagnostics if enumerated there
- [x] ~~`CLAUDE.md`~~ (N/A: implementation reuses the existing scheduler job
  and mail sender seams; no repository-wide convention was added)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** TKT-XWZIOB's RR-MAILI1/2/3 establish the stable
calendar occurrence, pending-idempotency, and authority-reload boundaries reused
here. Mail adds no second expansion/delivery layer: the scheduler child is the
recipient-scoped action boundary, preventing double queueing and split retry state.
