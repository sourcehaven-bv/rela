---
id: RR-URYJXPF
type: review-response
title: 'Plan showed values hidden by visible:'
finding: The plan took Old values and Missing keys from raw rows and compared raw values, so a dry run showed or revealed by equality a property the principal may not see.
severity: critical
resolution: The gate now returns the redacted row and the plan uses it. A record that compares or writes a property in Redacted is a record error; a row hiding its key or a scope property is treated as unreadable (TestHiddenProperty_NotCompared, TestHiddenScope_NotNamed).
status: addressed
---
