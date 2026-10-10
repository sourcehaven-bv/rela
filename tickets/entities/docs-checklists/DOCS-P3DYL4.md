---
id: DOCS-P3DYL4
type: docs-checklist
title: 'Docs: OpenAPI spec complete enough to drive rela-server from restish'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: readUploadBody (two encodings, name sources, why a compressed body is refused), the generator's generation counter, the servers "/" choice, why the SSE and git routes are left out of the spec, the CSRF exemption entry for _openapi.json
- [x] Function/type docs if public API: openapi.SecurityScheme, SecurityRequirement, Encoding, Config.AuthHeader, Generator.SetAuthHeader, attachment.DisplayName

## Project Documentation

- [x] README updated (if applicable): docs table links the new restish guide
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new pattern; the change follows the existing non-browser exemption and attachment service rules)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no rela CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog; release notes come from commits)
- [x] API docs updated (if applicable): docs/data-entry/api-reference.md (raw-body upload, name sources, 415, size cap without Content-Length); docs/server-security.md (non-browser exemption section rewritten, RFC 9728 note corrected); new docs/restish.md (setup behind an OAuth proxy, upload and download)
