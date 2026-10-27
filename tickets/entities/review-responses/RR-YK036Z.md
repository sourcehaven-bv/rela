---
id: RR-YK036Z
type: review-response
title: Type checks are incomplete
finding: min/max type equality, enum identity for pick, where value validity and multi-select any-element rule.
severity: minor
resolution: rollup.Compile checks min/max/sum/latest_by/pick types against the declared type (enum value sets must match), validates where values against enums and booleans, and refuses list-valued where keys.
status: addressed
---
