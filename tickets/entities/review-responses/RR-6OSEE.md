---
id: RR-6OSEE
type: review-response
title: Hyphenated entity IDs emitted unquoted in schema DOT output
finding: '`runSchemaGraphviz` emits `%s [label=...]` and `%s -> %s [...]` with unquoted entity identifiers. DOT''s unquoted identifier grammar forbids hyphens; the tickets metamodel has many hyphenated types (automated-measure, bug-analysis-checklist, planning-checklist, review-response, etc.). Reproduced: `RELA_PROJECT=./tickets rela schema --graphviz | dot -Tpng` → `syntax error in line 7 near ''-''`. This is the same class of bug as PR-522 (cluster IDs in `rela graph`) but in the sibling `rela schema` codepath. No test catches it because every fixture uses synthetic non-hyphenated names.'
severity: critical
resolution: 'Added `dotID()` helper in internal/cli/schema.go that quotes identifiers when they aren''t safe for DOT''s unquoted grammar. Every entity ID emission in runSchemaGraphviz now goes through dotID. Added TestDotID (table test) + TestSchemaGraphvizHyphenatedIDs which builds a metamodel with hyphenated types, asserts no unquoted hyphens leak, and — when graphviz is in PATH — pipes the output through `dot -Tdot` for a true parse-validity check. Manually verified against tickets/metamodel.yaml: `RELA_PROJECT=./tickets rela schema --graphviz | dot -Tpng` now renders cleanly (892 KB PNG).'
status: addressed
---
