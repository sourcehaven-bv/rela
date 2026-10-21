---
id: RR-C16SOF
type: review-response
title: Pin adapter field coverage by reflection
finding: 'The adapter comment already warns that every claim the adapter forgets is a silent widening. TKT-OJL2GN and TKT-IAC8TX both fixed a forgotten claim. ExpiresAt is a third field that fails silently the same way: if the adapter forgets it, streams never end. AC4 adds a targeted test for this field only. Consider a reflection test in cmd/rela-server instead. It builds claims with every AssertionClaims field set to a non-zero value and asserts that every AssertedIdentity field comes out non-zero. A new field then fails the test until the adapter copies it.'
severity: nit
resolution: 'Plan updated: a reflection test checks that the adapter sets every AssertedIdentity field.'
status: addressed
---
