---
id: RR-MK1DKEG
type: review-response
title: "empty_message has no Go test and no configure field"
finding: "Nothing pins the YAML/JSON names, and the configure screen cannot set it."
severity: minor
resolution: "TestList_EmptyMessage pins YAML and JSON; the configure list screen has a Text when empty field, read by listModel and covered in screenEdits.test.ts; the allowlist accepts lists.*.empty_message."
status: addressed
---
