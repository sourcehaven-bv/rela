---
id: RR-XAMB12V
type: review-response
title: Wire text changes for an undeclared current value
finding: 'enforce.go: a move from a value the enum no longer declares changed its message from "is not a declared transition" to "the current value is not a declared state" and printed from twice.'
severity: minor
resolution: 'Addressed: the message keeps "is not a declared transition" and appends the undeclared state in parentheses; the error class is unchanged.'
status: addressed
---
