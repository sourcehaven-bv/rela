---
id: RR-NSUN49
type: review-response
title: MCP prompts are a third ungated read surface, embedding entity data into LLM-facing text
finding: internal/mcp/prompts.go has 7 raw deps.Store/deps.Tracer reads (lines 68, 80, 130, 132, 210, 238, 287). Prompt bodies embed entity data and orphan lists directly into text handed to an LLM, so gating tools and resources while leaving prompts raw preserves the leak in the least obvious place.
severity: critical
status: open
---

## Finding

The remote-MCP plan's read-gating covered tool handlers. Design review found
`internal/mcp/prompts.go` is a third parallel read surface, with 7 raw reads:

- `prompts.go:68`, `:132`, `:210`, `:287` — `s.deps.Store`
- `prompts.go:80`, `:130`, `:238` — `s.deps.Tracer` (incl. `FindOrphans`)

Prompts (`analyze-traceability`, `review-orphans`, `summarize-project`,
`review-entity`) render entity data and orphan lists **into the prompt text
returned to the client**. Over a remote transport this hands an unauthorized
caller graph content in the least-inspected code path — and unlike a tool
result, prompt text is designed to flow straight into an LLM context.

## Resolution required

1. Route prompt handlers through the same injected visibility-wrapped
reader/tracer as tools and resources.
2. Alternatively, if prompts are judged low-value remotely, exclude them from
the remote server registration (the same treatment as Lua tools) — but that must
be an explicit, test-pinned decision, not an oversight.
3. Extend AC #3 to cover the prompt surface.
