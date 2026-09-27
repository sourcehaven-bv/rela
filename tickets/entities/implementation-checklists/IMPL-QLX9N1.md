---
id: IMPL-QLX9N1
type: implementation-checklist
title: 'Implementation: MCP attachment tools: list/read/attach/delete for agents, ACL-gated'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (`internal/mcp/tools_attachment_test.go`: TestAttachmentContent table; `internal/attachment/attachment_test.go`: TestService_DetachFile, TestService_StampPreservesOtherProperties)
- [x] Integration tests written (MCP handlers over a real appbuildtest bundle and the production `GatedReads` seam; TestAttachments_HTTPAcceptsLargeUpload posts a 5 MiB upload through `HTTPHandler`)
- [x] Happy path implemented (list/read/attach/delete; stdio and remote wiring)
- [x] Edge cases from planning handled (max 1 replace, suffixing, at-capacity, inline read cap, upload cap, invalid base64, non-file/undeclared property, path in file_name, locked entity, idempotent delete, ambiguous unnamed delete, concurrent writers)
- [x] Error handling in place (store errors logged and answered generically; policy errors named; rejections and denials audited)

## Test Quality

- [x] Using fixture builders or factories for test data (`newAttachFixture`, `attachOpts`, `aclPolicy`)
- [x] No hardcoded values in assertions when object is in scope (assertions compare against the attach result path and seeded ids)
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (`bin/rela mcp` over stdio against a scratch project in `.ignored/mcp-attach-verify`)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Scratch schema: `doc` with `spec: file` and `shots: {file, max: 2, accept:
[image/png]}`.

- attach_file notes.md to spec: returned `{"path":"attachments/DOC-1/spec/notes.md"}`; read_attachment returned text `# Notes\nhi`.
- attach_file a.png to shots: stored; entity frontmatter `shots: [attachments/DOC-1/shots/a.png]`; read_attachment returned image content `image/png`.
- attach_file b.txt to shots: `attachment rejected: content type "text/plain" is not in the allowed list`; audit log has `denied-write` "rejected upload ... (op=attachment-write)".
- list_attachments: `[{"property":"shots","file_name":"a.png","content_type":"image/png","size":24}]`.
- delete_attachment spec (unnamed): "Removed notes.md from DOC-1.spec"; frontmatter `spec: ""`. Repeat with file_name: "notes.md was not attached to DOC-1.spec; nothing to remove".
- read_attachment with file_name `../../schema.yaml`: "file_name must be a plain file name, not a path".
- ACL (hidden vs absent, denied write keeps bytes and is audited, `visible:`-hidden property) is covered by the TestAttachments_ACL_* tests through the production gated reader.

## Quality

- [x] Code follows project patterns (consumer-side interfaces, handler group + selector, snapshot once per call, PatchEntity for partial writes)
- [x] Checked for DRY opportunities (audit record builders shared with the web path in `internal/audit/attachment.go`; `attachment.RejectionReason` shared)
- [x] No security issues introduced (gated read first, `update` preflight before bytes, same processor as web/CLI, separators refused in file_name)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
