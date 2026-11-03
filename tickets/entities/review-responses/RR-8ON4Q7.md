---
id: RR-8ON4Q7
type: review-response
title: Unattenuated clients not checked in the dataentry e2e test
finding: The e2e test did not show that a client outside the baseline still reads the field.
severity: minor
resolution: TestCeilingE2E_FieldCeilingWithoutRoleFieldGrants now also reads as an unattenuated client and expects the field.
status: addressed
---
