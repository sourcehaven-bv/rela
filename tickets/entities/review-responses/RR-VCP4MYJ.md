---
id: RR-VCP4MYJ
type: review-response
title: Whitespace-only env user suppresses the warning
finding: envUser == '' tests the raw value while the resolver trims it, so RELA_DATAENTRY_USER=' ' gave no warning yet every request failed; validateConfigEditing had the same gap.
severity: minor
resolution: hasIdentitySource trims the value; test cases for a blank env user in TestWarnUnidentifiedACL and TestValidateConfigEditing.
status: addressed
---
