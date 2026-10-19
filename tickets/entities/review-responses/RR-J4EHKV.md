---
id: RR-J4EHKV
type: review-response
title: PropsSchema agreement claim (AC5) is overstated
finding: jsonschema-go treats format as an annotation by default, and the schema cannot express when:, MaxProps, the MessageBytes cap or ref type. "Accepted exactly when validateProps accepts" will fail on dates and sizes. Name the excluded checks in AC5 and assert format in the test validator.
severity: minor
resolution: AC5 names the checks the schema cannot express (when, MaxProps, MessageBytes, ref type). The test validator asserts format.
status: addressed
---
