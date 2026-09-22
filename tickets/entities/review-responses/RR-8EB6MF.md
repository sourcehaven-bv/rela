---
id: RR-8EB6MF
type: review-response
title: Test doubles embedded a GraphReader interface, so a zero value compiled and panicked later
finding: 'failingCountReader and failingListReader in internal/mcp/tools_cardinality_test.go embed the GraphReader interface. Both tests populate it correctly, but the embedding means `failingListReader{err: x}` compiles with a nil reader and every un-overridden method then panics on call — a stack trace pointing at the embedded interface, nowhere near the mistake. CLAUDE.md''s "constructors reject nil required fields" applies to test doubles for the same deferred-failure reason.'
severity: minor
resolution: Added newFailingCountReader(t, base, err) and newFailingListReader(t, base, err), which t.Fatal on a nil base, and a field comment on each type stating the requirement. Both call sites use the constructors.
status: addressed
---
