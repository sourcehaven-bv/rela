---
id: RR-52GIOIW
type: review-response
title: Late fill on a dirty form becomes a permanent unsaved change
finding: A dirty form was not re-baselined, so the late default alone kept it dirty.
severity: minor
resolution: Only the filled keys are patched into the baseline. Test asserts the baseline holds the late value and not the edit.
status: addressed
---
