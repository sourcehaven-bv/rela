---
id: DEC-HWZHA
type: decision
title: 'Validation policy for write APIs: tolerate temporarily invalid data'
context: |-
    rela's storage format is deliberately permissive: entity and relation files are markdown + YAML frontmatter, edited freely by external tools (text editors, IDE plugins, git merges, scripts) alongside the API. CLAUDE.md states the philosophy explicitly: "tolerate temporarily invalid data". Cardinality is documented as advisory. The `analyze_*` tools exist precisely to surface inconsistencies that the storage layer doesn't reject.

    Despite that, recent tickets (closed PR #648 / TKT-K2VAA, the original draft of TKT-6WLSW, design-review feedback on those) introduced hard 422 rejections at the API write path for: closed-schema unknown meta keys, target-type mismatches, missing target IDs, required-meta enforcement. None of those are structural impossibilities — they're inconsistencies of the kind `analyze` would flag. A user editing markdown by hand can produce all of them, and the API rejecting them on the next save creates a hostile asymmetry: the file system tolerates the state, the API doesn't.

    The drift had three contributing causes: (1) JSON:API §9 wire-shape adoption brought along JSON:API's "validate-then-write, return 422" mental model from REST-over-database stacks where wire and storage share a closed schema; (2) auto-save UX wants immediate per-field feedback, which crept into the wire layer instead of staying a UI concern; (3) code-review pressure ("is this validated?") tightens the strictness ratchet without anyone explicitly arguing for hard rejection.
consequences: |-
    ## Validation classification

    Write-time checks are split into three classes, with distinct HTTP behavior:

    **Hard 400 — malformed wire format**
    Reserved for request-structure errors detectable without consulting the metamodel. Examples: malformed JSON, missing required wire-format fields (`{"tagged": {}}` with no `data` key), mixing legacy and modern shapes in one body. These are caller bugs in the HTTP layer, not data-state issues.

    **Hard 422 — structural impossibilities**
    Reserved for things the storage layer literally cannot persist or where the in-memory model has nowhere to put the data. Examples: writing `content` to a relation type whose disk file format doesn't support a body (the file shape can't hold it), unknown relation type (no defined storage location), missing required wire-format fields like `id` or `type`.

    **Write-with-warnings (200 + warnings)**
    Everything else that today's drafts wanted to 422 on. The API performs the requested write and returns the warnings in the response so UIs can surface them non-blockingly. Subsequent `analyze` runs flag the same conditions. Examples: target-type mismatch against the relation's allowed `to` set, missing target ID, unknown meta keys (closed-schema violation), required-meta unset, target entity type not in `from` allowlist. These are all conditions a hand-editor can produce and rela's `analyze_*` tools already detect.

    ## Response shape additions

    Write endpoints that perform the work despite warnings return their normal 200 body augmented with a `warnings: []` array. Each warning is a structured object: `{code, path, detail}` where `code` is a stable identifier matching what `analyze_*` would surface, `path` is a JSON-pointer-style location, and `detail` is human-readable. Empty array (or omitted field) means no warnings.

    ## Scope of this decision

    Applies to all data-entry-server write endpoints (`POST /api/v1/{plural}`, `PATCH /api/v1/{plural}/{id}`, the per-edge relation endpoints, future bulk endpoints). The MCP tools and CLI follow the same policy: warnings flow back as part of the result, no hard rejections for soft conditions. Lua scripts get warnings via the existing return channel.

    ## Migration

    Existing endpoints that today hard-reject on soft conditions are NOT retroactively softened in this decision — each migration is its own ticket so existing callers aren't surprised. The decision sets the policy for new endpoints and for explicit rewrites.

    ## Out of scope

    Validation in the analyze tools, in CLI commands like `rela validate`, and in metamodel-loading itself. Those are appropriate places for hard policy checks because they're explicitly diagnostic surfaces. The decision is specifically about WRITE paths.
date: "2026-05-07"
status: accepted
---
