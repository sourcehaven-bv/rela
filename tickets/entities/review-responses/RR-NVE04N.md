---
id: RR-NVE04N
type: review-response
title: AC 10 cites a docs example test that does not cover data-entry.md
finding: 'docs_example_test.go exists only for realtimecfg (GUIDE-realtime.md) and acl. Nothing loads the YAML examples in docs/data-entry.md. Change AC 10: add the documented PoC example as a case in TestChannelsPresentation_Validate (channels_test.go:66), or drop the claim.'
severity: minor
resolution: AC 10 now tests the documented PoC example as a docs example case in TestChannelsPresentation_Validate, and the docs_example_test.go claim is dropped.
status: addressed
---
