---
id: IMPL-2OMQEV
type: implementation-checklist
title: 'Implementation: Lua rela.attach_render: render, convert and attach'
started: "2026-10-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (lua `attachrender_test.go`: binding, returned error, absent without renderer, eleven argument misuse cases; appbuild holder test; attachment `Result.Size`/`SHA256` test; `TestRenderFileName`; storetest `SHA256` subtest on every backend; sqlitedb and pgstore migration tests)
- [x] Integration tests written (test full flow, not just units) (`internal/dataentry/attach_render_test.go`: real action scripts, real Lua engine, real cmdexec transforms, Declarative ACL)
- [x] Happy path implemented
- [x] Edge cases from planning handled (hidden rows, hidden entity, missing entity, non-file property, permission document, elevated and command documents, unknown document/list/format, extension mismatch, entry on standalone, anchored without entry, rejected bytes, expired deadline, no bound identity, outside a request)
- [x] Error handling in place (errors surfaced, not swallowed) (refusals return nil and a message; argument misuse raises; conversion failures are logged and returned)

## Test Quality

- [x] Using fixture builders or factories for test data (`newAttachApp`, `attachMeta`, `attachPolicy`)
- [x] No hardcoded values in assertions when object is in scope (hashes compared to `sha256` of the stored bytes)
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Built `rela-server` for fs, sqlite and postgres with the SPA embedded. Project
in `.ignored/manual-ar`: `audit` with a `file` property, `finding`, a script
document over an audit, a findings list, a `docx` pandoc transform, an action
calling `rela.attach_render` twice (docx document, md list), and an `acl.yaml`
with alice (reads findings, updates audits), carol (cannot read findings,
updates audits) and dave (reads all, updates nothing). Principal via
`--principal-header X-Forwarded-User`.

Result, identical on all three backends:

- alice: `report-alice.docx` (docx MIME) and `findings-alice.md`; both contain
  the findings.
- carol: `report-carol.docx` and `findings-carol.md`; neither contains a
  finding.
- dave: `attach_render: permission denied to update "AUD-1": no role grants
  update on type "audit"`; nothing stored.
- For every file the `_attachments` `sha256` equals the SHA-256 of the
  downloaded bytes.
- Audit log has one `attach-render` record per file with principal, sha256,
  size and source (`document="audit_report" entry="AUD-1" format="docx"`).
- Postgres: a schedule `run_as: carol` attached `nightly.md` through the
  appbuild holder; its hash equals carol's interactive list render.
- A pandoc command without `-t docx` wrote HTML; the upload MIME policy
  refused it (`content type "text/html" is not allowed`).

## Quality

- [x] Code follows project patterns (check similar code) (consumer-side `lua.AttachRenderer`; transport-free cores shared with the HTTP handlers; package functions where App and Services sit at their plimsoll caps)
- [x] Checked for DRY opportunities (upload write check, document gates and list export pipeline extracted into cores the HTTP handlers and the renderer both call, rather than copied)
- [x] No security issues introduced (render as caller, no elevation, write through entitymanager with the upload policy; elevated documents refused)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
