---
id: RR-CP0NK5
type: review-response
title: Adapter reflection test misses swapped fields
finding: cmd/rela-server/main_identity_test.go:142-163 sets every string claim to "x" and checks only that each output field is non-zero. An adapter that copies OrgID into OrgSlug, or Subject into Email, still passes. That is the same silent-mapping bug class the test comment cites (TKT-OJL2GN, TKT-IAC8TX). Give each field a unique value (its field name for strings, a distinct time for ExpiresAt), then compare each output field to the input field with the same name.
severity: nit
resolution: The reflection test gives each field a unique value and compares each output field to the input field of the same name.
status: addressed
---
