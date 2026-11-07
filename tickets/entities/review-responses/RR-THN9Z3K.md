---
id: RR-THN9Z3K
type: review-response
title: expect_access needs the unexported accessExpectation from internal/cli
finding: cli/acl_expect.go:36,175; apptest cannot import cli.
severity: minor
resolution: Move the expectation type and comparison into internal/aclmap; cli and apptest both use it. Added to files to modify.
status: addressed
---
