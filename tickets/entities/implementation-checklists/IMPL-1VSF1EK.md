---
id: IMPL-1VSF1EK
type: implementation-checklist
title: 'Implementation: Attachments in channel messages'
started: "2026-11-10"
completed: "2026-11-10"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Manual, with the real binaries on a scratch project (`.ignored/manual-files`, a
`shots` channel with `image: { type: file, max: 2, max_size: 64KiB, accept:
[image/png] }`):

- CLI: `rela channel post ITEM-001 shots photo -P caption=hi --file image=dot.png`
posted #1; `read` printed `file image: dot.png (image/png, 69 bytes) id ...`;
`--schemas` printed `image: file, at most 2 of 65536 bytes, accept image/png`.
`rela channel file ... --to out.png` wrote identical bytes (cmp). A text file
named fake.png was refused: `file_refused: content type "text/plain" is not in
the allowed list`, and the audit log has a `denied-write` record `rejected
upload ... (op=channel-file-write)`. After `channel remove`, the download
answers `file_not_found` and no file bytes remain under `.rela/channels`.
- HTTP (`rela-server -principal-header X-User`): multipart post 201; text file
422 `file_refused`; file part before the message part 400; download 200 with
`Content-Disposition: attachment`, `Content-Type: image/png`, `nosniff`,
`Cache-Control: private, no-store` and identical bytes; unknown file id 404
`file_not_found`.
- Web app: Playwright `channels.spec.ts` "message files" (2 tests, also
`--repeat-each=3`): post an 8x8 PNG, live chip and `<img>` preview for a second
reader, download with identical bytes; a text file refused by the composer, a
renamed text file refused by the server, and the API refusal 422. The whole
channels spec passes (18). The e2e found that the browser named the JSON
`message` Blob part "blob", which the server refuses; the SPA now sends that
part as a string.

Automated: realtimecfg, realtime (files, shape, transform, refusal),
channelstest for file/sqlite (append with files, OpenFile, Remove, Delete,
Tombstone, Purge, Rekey), dataentry handler (multipart, download, busy limiter,
refusals), MCP (post with files, read_message_file, hidden anchor), CLI (post
--file, read, --schemas, file --to, errors), appbuild checker adapter, OpenAPI
paths; frontend vitest 6062 passing.

Deviations from the plan: multipart bodies are read into memory, bounded by the
16 MiB cap and the shared upload limiter, rather than spooled to disk. MCP
uploads rely on the transport's request size cap rather than the upload limiter.
`realtime.FileOpener` serves downloads apart from `realtime.Service` to keep the
service's API under its plimsoll line.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
