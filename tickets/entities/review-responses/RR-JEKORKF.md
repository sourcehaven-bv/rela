---
id: RR-JEKORKF
type: review-response
title: 'The dataentry test package did not compile'
finding: 'v1.FieldError gained a slice field, so the write refusal test compared structs with !=.'
severity: critical
resolution: 'The test compares with reflect.DeepEqual.'
status: addressed
---
