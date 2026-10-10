---
id: RR-SX0GQ8
type: review-response
title: Scalar typing and aliases unspecified
finding: Strings like '1.0', 'yes', 'null' or dates must stay strings; comparisons must use decoded values; null vs absent undefined.
severity: significant
resolution: 'Addressed in code and plan: new strings carry an explicit !!str tag (the encoder quotes them), comparisons use the decoded node value, null is a value and absence is a removed key; anchors, aliases and merge keys are refused (configedit/tree.go).'
status: addressed
---
