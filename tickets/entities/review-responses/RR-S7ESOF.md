---
id: RR-S7ESOF
type: review-response
title: inspect's exhaustiveness guard had no test
finding: 'The default: panic in Program.inspect is correct, but no test compiled one expression per sealed node type, so relational, concat, arithmetic, unary-minus and table-argument nodes were reached incidentally at best and a new node type could still land untested.'
severity: minor
resolution: TestProgram_InspectCoversEveryNodeType compiles one expression per node type (bool profile for const/var/attr/relational/logical/not/call/table-arg; ValueProfile for arithmetic, unary minus and concatenation) and reads References/Functions on each.
status: addressed
---
