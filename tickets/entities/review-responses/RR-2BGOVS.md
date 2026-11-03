---
id: RR-2BGOVS
type: review-response
title: Stdio add_comment maps errors inconsistently
finding: Service validation errors came back raw on stdio but as invalid_comment on remote.
severity: minor
resolution: comments.Refusal classifies Service errors; dataentry failComment and stdio AddComment both use it.
status: addressed
---
