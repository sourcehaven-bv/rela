---
id: RR-T0DT7G7
type: review-response
title: Title test mutates the parsed tree
finding: The second case edits Children[0].Children[0].Children[0], which breaks silently if the fixture changes. Use a table of fixtures.
severity: nit
resolution: Rewritten as a table of three fixtures; no tree mutation.
status: addressed
---
