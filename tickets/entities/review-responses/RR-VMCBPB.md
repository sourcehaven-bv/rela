---
id: RR-VMCBPB
type: review-response
title: Untyped read returns a gate error only for existing ids
finding: '[security] addressAny/familyAny ran the gate only after the raw stored-type header read found the id; so a gate error (unstamped principal; MatchingIDs failure) came back only for existing ids and revealed existence and type (Lua rela.get_entity raise vs nil; MCP attachment error text; Lua search hydration).'
severity: significant
resolution: addressAny and familyAny now log a gate failure (warnGate) and answer the uniform miss. ScriptReader.GetEntity doc updated. Pinned by TestScriptReader_GateErrorIsAMiss.
status: addressed
---
