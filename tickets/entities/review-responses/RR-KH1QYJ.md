---
id: RR-KH1QYJ
type: review-response
title: MCP budget test runs no real gate
finding: TestBuildStoreRelations_ReadBudget goes through Unrestricted (NopGate) with one type, so the gated ScriptReader path is not pinned.
severity: minor
resolution: 'Added TestScriptReader_ResolveHeadersBudget: ScriptReader over a PolicyReader with a counting gate, two types, one header read and two PermitsReadMany at 10 and 50.'
status: addressed
---
