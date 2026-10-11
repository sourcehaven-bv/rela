---
id: RR-1IXAQ5
type: review-response
title: Editable leaf replaced by a container was not checked
finding: diff returned at an editable scalar path without looking inside a mapping or list put in its place.
severity: minor
resolution: A container replacing an editable scalar is checked as an addition. Allowlist test case added.
status: addressed
---
