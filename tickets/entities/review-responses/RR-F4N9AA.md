---
id: RR-F4N9AA
type: review-response
title: Untyped route cache grows per property name for no benefit
finding: Every undeclared property routes to text, so a Map keyed by name stores the same route repeatedly.
severity: minor
resolution: Replaced with one lazily resolved text route.
status: addressed
---
