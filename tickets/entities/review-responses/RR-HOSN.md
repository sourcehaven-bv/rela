---
id: RR-HOSN
type: review-response
title: Inaccessible-entity rewrite skipped when type is empty — unreachable, but the check is brittle
finding: 'frontend/src/utils/markdown.ts lines 85-86: `if (!hit || !hit.type) return; if (!hit.inaccessible && !hit.title) return;` The first guard rejects when the resolver returns an entity with no type. On the data-entry path the server always sets Type (server-side Mention.Type comes from ent.Type, a required Entity field) so this branch is unreachable today. But the guard order conflates two intents: ''malformed Mention'' (no type) versus ''nothing to display'' (no title and not inaccessible). For a partially-locked entity where the type is set, inaccessible=true but title='''', the code falls through correctly. Suggest separating these into early returns for the malformed/inconsistent case (warn in a test) and a clean ''nothing to do'' fall-through. Minor, but the comment ''Require a type so we can build a stable URL'' is the kind of one-line load-bearing assumption that breaks loudly when the next dev changes the resolver shape.'
severity: nit
resolution: 'rewriteEntityRefToken refactored: type must be present (else cannot build URL); title may be empty for inaccessible (falls back to ID); for non-inaccessible a missing title means ''resolver doesn''t know enough''. Conditions inlined into a visibleTitle computation so precedence is readable.'
status: addressed
---
