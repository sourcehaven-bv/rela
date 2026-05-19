---
id: RR-KJMU
type: review-response
title: TestHeaderPrincipalResolver_ToolUnchanged missed chain fallback
finding: The "default" case used defaultPrincipalResolver directly, which always returns Tool=ToolDataEntry. It didn't exercise the chain's own fallback Tool — a regression in ChainResolvers calling something else would slip past.
severity: significant
resolution: 'Added a "chain-fallback" subtest using ChainResolvers(HeaderPrincipalResolver("X-User")) with no env and no header, asserting Tool comes out as ToolDataEntry. Pins the chain''s fallback path. File: internal/dataentry/principal_test.go:240-262.'
status: addressed
---
