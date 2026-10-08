---
id: RR-ACIIJV
type: review-response
title: Quote split via goldmark parse is fragile
finding: Parsing a source fragment out of context misparses ordered lists starting >1, tables without delimiter row, indented continuation as code, unclosed fences; also adds a parser to the resolve path that collapse.go keeps parser-free.
severity: significant
resolution: 'Plan revised: resolve path splits quote and document with splitParagraphsWithOffsets (blank-line chunks); no parser on the resolve path; endpoint matching instead of AST alignment.'
status: addressed
---
