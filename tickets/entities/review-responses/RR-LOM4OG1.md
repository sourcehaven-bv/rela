---
id: RR-LOM4OG1
type: review-response
title: Nothing marks the run store as off-limits to read-out packages
finding: Store.List and Get return run state for any subject id; only a package doc says read paths must go through the _flow-run row gate.
severity: minor
resolution: The flowrun arch-lint component now states that only the runtime and composition root may depend on it and read-out packages read runs through the _flow-run row gate. dataentry, mcp, cli and openapi have explicit mayDependOn lists, so adding flowrun there needs a reviewed arch-lint edit. An archguard allowlist is left to TKT-9IT3LN8, which adds the first importer.
status: addressed
---
