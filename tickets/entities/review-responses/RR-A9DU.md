---
id: RR-A9DU
type: review-response
title: 'Leverage: extract a shared ''entity-ID code span resolution'' contract between Lua and HTTP path'
finding: 'The Lua-side path (internal/lua/markdown.go rela.md.entity_refs / resolve_refs) and the new data-entry path (internal/dataentry/mentions.go) implement the same conceptual operation: ''scan markdown for bare-content code spans whose text matches an entity ID; emit a resolution map''. Today they diverge slightly: Lua uses titleSlug-style anchors, data-entry uses /entity/<type>/<id> URLs. Both have their own AST walk, both have their own ''must be bare-content'' rule, both have to be kept in sync as the markdown surface grows. As the platform adds more rendering paths (e.g. RSS feed, doc export, MCP resource exposure), this duplication compounds. Suggest factoring a shared `internal/mentions` (or `internal/refscan`) package that exposes ScanCandidates(content) []string and lets each caller decide what to do with the resolution. Out of scope for this PR — file it as a follow-up ticket.'
severity: nit
reason: 'Factoring a shared internal/refscan between Lua-side entity_refs and HTTP-side collectMentions is useful leverage but out of scope for this ticket. The two paths differ in how they consume the result (Lua: project-wide map; HTTP: per-response scope) and consolidating now expands blast radius beyond what this PR delivers. Test suites for both paths cover the shared semantics independently.'
status: deferred
---
