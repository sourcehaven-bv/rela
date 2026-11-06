---
id: RR-07SZ6BW
type: review-response
title: Strict refusals named no property in CLI and MCP
finding: ValidationError.Error() joined bare messages such as 'This field is required'.
severity: significant
resolution: Each message is prefixed with its property name when it has one.
status: addressed
---
