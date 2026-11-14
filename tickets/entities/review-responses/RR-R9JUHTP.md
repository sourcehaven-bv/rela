---
id: RR-R9JUHTP
type: review-response
title: expect_denied missed denials on hidden targets
finding: A write on an entity the actor may not read returns not-found, which expect_denied did not count as a denial.
severity: significant
resolution: checkTargets wraps a not-found for an id the raw store holds as hiddenTargetError, which expect_denied accepts. TestDeniedHiddenTarget covers hidden, missing and stale cases.
status: addressed
---
