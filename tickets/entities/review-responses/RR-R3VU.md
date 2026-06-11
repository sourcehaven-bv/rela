---
id: RR-R3VU
type: review-response
title: Test scripts hardcode IDs coupled to newMockWorkspace seeds
finding: 'runtime_test.go:2344-2350: scripts reference TKT-001, FEAT-001, and ''Test'' — values seeded in newMockWorkspace. If seeds change, the search subtest could silently no-op (zero hits) while still passing. Per CLAUDE.md ''Avoid Hardcoded Values'', extract via a helper or document the coupling.'
severity: nit
resolution: Lifted hardcoded IDs to const declarations (ticketID, featureID, searchTerm) at the top of the test and used fmt.Sprintf in each script. Added sanity-check ws.GetEntity assertions at the start of each subtest that t.Fatal if the seed is missing — so any future seed rename produces a loud failure instead of a silent no-op. The IDs and search term still match the mock seeds (which is intentional — they're injected trigger values per CLAUDE.md), but the coupling is now both centralized and verified.
status: addressed
---
