---
id: RR-Y8PRYR
type: review-response
title: Test-3 comment claimed validation rules the test cannot verify
finding: TestQueryBudget_FixtureConfigIsValid's comment listed four nested-section validation rules (children differs from source, children rule traverses from source, recursive refused, columns refused). The test only calls ValidateConfig on a VALID config, so it confirms no rule rejects the fixture but verifies none of those rules exist - if all four were deleted it would still pass.
severity: minor
resolution: Comment trimmed to what the test actually asserts (validation accepts these fixtures) and now points at TestValidateConfig_NestedSection in internal/dataentryconfig, which exercises the rules against invalid input. Verified that test exists at validate_test.go:3067.
status: addressed
---
