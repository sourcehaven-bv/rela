---
id: RR-9W7IA1
type: review-response
title: Zero-value MCPActions in the wiring test
finding: The wiring test uses &dataentry.MCPActions{} whose nil handler would panic if invoked.
severity: nit
reason: The test checks wiring only and never invokes a tool, as the comments test does with MCPComments{}.
status: wont-fix
---
