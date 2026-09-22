---
id: RR-VUSO7Z
type: review-response
title: handleAnalyzeCardinality read deps() twice in one expression
finding: The new handler line was `schema.CheckCardinality(ctx, s.deps().Store, s.deps().Meta, nil)` — two separate atomic.Pointer loads of the dependency bundle in one expression. Server.ReloadDeps republishes the whole bundle on a schema.yaml edit, so a reload landing between the two loads pairs a new store with an old metamodel, or vice versa. This violates CLAUDE.md's "Capture state once per operation" rule, which names MCP tools explicitly.
severity: significant
resolution: |-
    Take one snapshot (`d := s.deps()`) at the top of the handler and read both fields from it, with a comment naming the reload seam as the reason.

    Note the same defect exists at two other sites in the same file (handleAnalyzeUnique and handleAnalyzeProperties) and predates this ticket; left alone here rather than widening scope.
status: addressed
---
