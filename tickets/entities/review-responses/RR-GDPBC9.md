---
id: RR-GDPBC9
type: review-response
title: Cardinality message rendering was triplicated across CLI analyze, CLI validate and MCP
finding: 'A ticket whose purpose was "one cardinality check, shared" left the RENDERING in three places: internal/cli/analyze.go:218, internal/cli/validate.go:212, and the new cardinalityMessage helper in internal/mcp/tools_analysis.go each independently re-derived "min_ prefix -> ''at least'' phrasing". The strings.HasPrefix(v.Constraint, "min_") test is itself a smell: Constraint is a closed set of four values.'
severity: significant
resolution: |-
    Hoisted onto the shared type in internal/schema/cardinality.go as CardinalityViolation.IsMin() and CardinalityViolation.Message(). Message() deliberately omits the entity id, because its placement differs per surface — the CLI prefixes it, MCP carries it in a sibling JSON field — and the godoc says so. All three call sites now render through it; the local cardinalityMessage helper is deleted.

    CLI output verified byte-identical for both the min and max branches before and after, via a throwaway comparison test against the old format strings.

    Not done: turning Constraint into a typed enum. That changes the JSON wire format of the violation and touches every consumer, which is beyond this ticket; IsMin() gives the three renderers the discrimination they needed without it.
status: addressed
---
