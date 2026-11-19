---
id: RR-TMPGOJI
type: review-response
title: Permission with surrounding spaces accepted
finding: A permission value with leading or trailing spaces was accepted and could never match a declared permission.
severity: minor
resolution: UnmarshalYAML refuses a permission with surrounding spaces. Test case 'permission with spaces'.
status: addressed
---
