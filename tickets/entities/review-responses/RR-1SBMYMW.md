---
id: RR-1SBMYMW
type: review-response
title: Bool encoding
finding: 'on: true lands in a string field; PATCH must send a JSON bool.'
severity: minor
resolution: Validation requires true/false for boolean; _toggles sends a JSON bool for boolean properties.
status: addressed
---
