---
id: RR-JCSIM1
type: review-response
title: mermaid Label did not escape double-quotes/brackets (injection contract broken)
finding: internal/mermaid rendered node/state text via fmt %q, which emits Go-style backslash escaping (\") that mermaid does NOT honor — mermaid uses HTML entities. A double-quote in a value/label broke out of the quoted label (e.g. n0["a"] --> x["b"]), re-entering mermaid's tokenizer as structure. The package's whole purpose is injection-safety and its docstring claimed to handle breakout chars, but the equally-breaking " was unhandled. Existing injection tests only asserted newline flattening, giving false confidence.
severity: significant
resolution: 'Label now entity-escapes \" → #quot;, < → #lt;, > → #gt; (mermaid''s decoded-entity form) in addition to flattening newlines; a new quoted() helper wraps escaped text and all three render sites (state alias, node text, edge label) use it instead of %q. Added TestLabel_EscapesQuotesAndBrackets, TestGraph_QuoteInNodeTextDoesNotBreakOut, TestStateDiagram_QuoteInValueDoesNotBreakOut that feed a bare quote and assert entity-escaping + exactly-two-delimiter-quotes.'
status: addressed
---
