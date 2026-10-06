---
id: RR-JAL80Q
type: review-response
title: Panic test covers only the before-write branch
finding: Inverting the !sw.written check would pass all tests.
severity: minor
resolution: Table test adds the after-write case expecting status=201 panic=true.
status: addressed
---
