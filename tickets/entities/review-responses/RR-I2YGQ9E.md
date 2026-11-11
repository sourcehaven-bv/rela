---
id: RR-I2YGQ9E
type: review-response
title: Example schema name in the upgrade guide is not a SHA-256
finding: org_5d41402abc4b2a76 is MD5('hello'), so a reader comparing step 1 with tenant show gets a mismatch.
severity: minor
resolution: The guide uses org_822b33ad87c148a0, the derived name for org id acme, throughout.
status: addressed
---
