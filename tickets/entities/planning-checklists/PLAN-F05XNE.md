---
id: PLAN-F05XNE
type: planning-checklist
title: 'Planning: OpenAPI spec complete enough to drive rela-server from restish (attachments + auth)'
started: "2026-10-09"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** an agent cannot upload a large attachment to a remote rela-server
without writing the file as base64 through the model (Atlas TASK-2ZX3I). restish
v2 (rest.sh) builds a CLI from an OpenAPI spec, handles OAuth, and sends a raw
file body from `@file` when an operation accepts `application/octet-stream`.
rela's spec is not usable by it today:

- restish cannot fetch it: `/api/v1/_openapi.json` gets 403 `origin_missing` from
  the CSRF check for a client without an Origin header, and restish's discovery
  (`{base}/openapi.json`, `Link: rel=service-desc`) finds nothing.
- The spec lists five legacy `/api/` paths that 404, has no `servers`, no
  security scheme, no `PUT`, no attachment operations, and describes errors as
  `application/json` instead of `application/problem+json`.
- The upload route accepts multipart only.

**Scope (in):**
1. Discovery: CSRF exemption for the spec, a `Link: rel=service-desc` header on
   `/api/v1/` responses, `servers` filled from the request.
2. Spec correctness: remove the drifted legacy paths; errors as
   `application/problem+json` with the full `v1.Error` shape; entity schema gains
   `_title`, `_attachments`, `_redacted`.
3. Attachments: per type with `file` properties, `PUT .../_attachments/{property}`
   (raw `application/octet-stream` + `multipart/form-data`), `GET`/`DELETE
   .../_attachments/{property}/{fileName}`. `{property}` is an enum of that type's
   file properties. Raw upload takes the name from `?filename=` or
   `Content-Disposition`.
4. Handler: raw-body branch in `handleV1PutAttachment`, same preflight, limiter,
   size cap, policy, audit as multipart.
5. Auth: a `securitySchemes` entry matching the JWT gate (`http bearer` when the
   header is `Authorization`, else `apiKey` in that header). No OAuth metadata:
   a local spike (Pratique v26.10.0 + restish 2.3.0) showed restish needs the
   spec before it can read auth from it, and the spec sits behind login, so the
   OAuth profile is configured by hand (documented; devops PR #267).
6. A guide: connecting restish to a rela-server.

**Scope (out):** OAuth flags/metadata in the spec (see item 5); spec coverage for views, history, comments, search, export,
documents and other `_` routes (follow-up tickets); RFC 9728 protected-resource
metadata; any rela-side OAuth server or token issuance; a restish plugin.

**Acceptance Criteria:**
1. `restish api connect atlas https://<host>/api/v1` discovers the spec and
   lists per-type commands. Test: e2e with restish against a local rela-server.
2. restish with a hand-configured OAuth profile logs in through Pratique and
   calls an entity GET. Verified in the local spike (works today); re-run after
   the change. Unit test on the emitted security scheme.
3. `restish atlas put-<type>-attachment <id> <prop> --filename shot.png @shot.png`
   attaches a 400 kB PNG byte-identical. Test: Go handler test (raw body) + e2e.
4. A raw upload passes the same ACL, MIME filter, size cap and audit as
   multipart: denied write → 403 + `AttachmentWriteDenied`; disallowed MIME →
   422 + `AttachmentRejected`; over limit → 413. Test: table-driven handler tests.
5. The spec validates as OpenAPI 3.1 with libopenapi (the parser restish uses),
   and every path+method in it resolves to a registered route (no 404/405).
   Test: generator test + router cross-check test.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A: the option survey happened in the session that
superseded TKT-YAROJX (signed URL, CLI-as-MCP-client, proxied services,
generated client, restish); summarized under Approach/Alternatives.

**Existing Solutions:**
- restish v2.3.0 (2026-06, actively maintained): OpenAPI 3.0/3.1 via
  pb33f/libopenapi; raw binary body for `application/octet-stream` operations;
  multipart via `encoding`; maps `securitySchemes` (bearer, apiKey, oauth2
  authorizationCode/clientCredentials/deviceAuthorization) to profiles;
  `x-cli-config` pre-fills profiles; `x-cli-name`/`x-cli-ignore` shape commands.
  Does not map `openIdConnect`. Discovery: `Link` service-desc/describedby,
  `/openapi.json`, same-origin only.
- `internal/openapi` generator (types.go, paths.go, schemas.go, generator.go).
- `feedBaseURL` (feed_handler.go:109) for a request-derived base URL.
- `attachment.Service.WriteAttachment` takes `(fileName, io.Reader)` and spools
  itself, so the raw branch reuses ACL, policy and audit unchanged.
- `isNonBrowserExemptV1Path` (middleware_security.go:319) already exempts
  `/api/v1/{plural}/...` and `/api/v1/_schema` for non-browser clients.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
1. `openapi/types.go`: `PathItem.Put`; `Spec.Security`, `Operation.Security`;
   `Components.SecuritySchemes`; `MediaType.Encoding`; `Spec.Extensions` for
   `x-cli-config`.
2. `openapi.Config` gains `AuthHeader` (the JWT gate header). Generator emits
   the matching scheme and a top-level `security`. Servers are not cached: `handleV1OpenAPI` sets them
   per request from `feedBaseURL(r) + "/api/v1"` on a copy, so the schema-hash
   cache stays valid.
3. `paths.go`: drop the five legacy paths and their tests; add attachment
   operations (operationIds `put<Type>Attachment`, `get<Type>Attachment`,
   `delete<Type>Attachment`) only for types with file properties; error
   responses reference `application/problem+json`.
4. `schemas.go`: `Attachment` schema, `_attachments`, `_title`, `_redacted` on
   the entity; `Error` gains `conflicts`, `versions`, `faces`.
5. `handlers_attachment.go`: branch on media type; raw branch uses
   `MaxBytesReader(limit)` + `store.CapAttachmentReader`, name from
   `?filename=` or `Content-Disposition`, 400 `missing_filename` otherwise.
6. `middleware_security.go`: add `/api/v1/_openapi.json` to the non-browser
   exemption (read-only config, not data, per CLAUDE.md "configuration is not a
   secret"). It stays behind the JWT gate.
7. `Link: </api/v1/_openapi.json>; rel="service-desc"` on `/api/v1/` responses.
8. Pass the JWT header name from the gate config into `openapi.Config`.

**Files to modify:** internal/openapi/{types,generator,paths,schemas}.go +
tests; internal/dataentry/{app.go,api_v1.go,handlers_attachment.go,
middleware_security.go} + tests; go.mod (libopenapi,
test only); docs/data-entry/api-reference.md, docs/server-security.md, new
docs/guides/restish.md; e2e test.

**Alternatives rejected:**
- Signed upload URL (TKT-YAROJX): new token mechanism, KV change, CISO review.
- CLI as MCP client: needs OAuth client code in rela; restish already has it.
- Proxying core services remotely: breaks the entitymanager write path and ACL.
- A Go client generated at compile time: the spec is per schema.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- Raw body: capped by `MaxBytesReader` + `CapAttachmentReader` at the
  property/app limit; content checked by the property's MIME allowlist and scan.
- File name from `?filename=` / `Content-Disposition`: parsed with
  `mime.ParseMediaType`, then the same sanitizing as a multipart name (to verify
  in `attachment.Service`; add it there if missing).
- `Host` / `X-Forwarded-Proto` for `servers`: `feedBaseURL` rejects unsafe hosts.

**Security-Sensitive Operations:**
- CSRF exemption for the spec: GET only, returns config, still JWT-gated; same
  conditions as the existing non-browser exemption (no Origin/Referer/Cookie/
  Sec-Fetch-Site).
- Raw upload: no new auth path; preflight, ACL re-check under lock, audit are
  the existing ones. A browser cannot send a cross-site raw PUT without CORS
  preflight, and the CSRF check still applies to browser requests.
- OAuth values in the spec are public (client id, endpoints); no secret.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** AC1: e2e script with restish (skipped when not installed)
against `rela-server` on a demo project. AC2: generator unit tests per auth
config; manual login against Atlas. AC3/AC4: table-driven handler tests (raw,
multipart, both name sources). AC5: libopenapi validation of the spec for the
demo and tickets schemas; router cross-check test walking every spec operation.

**Edge Cases:** raw body with no name (400); name in both query and header
(query wins, documented); empty body (rejected as today); body exactly at the
limit (accepted) and one byte over (413); `Content-Type` with parameters;
`ID@face`; a type with no file property (no attachment operations); a schema
reload changes the spec (hash invalidation).

**Negative Tests:** hidden entity (404 uniform), hidden property (404), locked
entity (422), ACL deny (403 + audit), MIME reject (422 + audit), busy limiter
(503), spec fetched by a browser-like request with a foreign Origin (403).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Auth in front of Atlas: resolved. Pratique accepts the OAuth bearer on every
  path and injects `Authorization: Bearer <assertion>`; verified locally.
- restish details unverified by running it (raw body, `@file` UTF-8 coercion).
  Mitigation: e2e test with a binary PNG early in implementation.
- Spec size grows per type; restish caches the spec, so acceptable.

**Effort:** l

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** docs/data-entry/api-reference.md (raw upload, spec
location), docs/server-security.md (OAuth flags, fix the stale
`resource_metadata` and curl/Origin text), new docs/guides/restish.md.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: user directed to proceed; the auth and discovery design was validated by the local Pratique + restish spike instead)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run, see above)

**Design Review Findings:** none; spike findings folded into Scope item 5 and Risks.
