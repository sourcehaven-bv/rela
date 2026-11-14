---
id: RR-JE23B3A
type: review-response
title: Data-entry create still compares manual id prefixes case-sensitively
finding: validateCreateIDOpts (api_v1.go) used strings.HasPrefix, so the HTTP create refused req-7 for a REQ- type that the write validator now accepts.
severity: significant
resolution: validateCreateIDOpts folds the prefix with strings.EqualFold and still requires a suffix. Two TestValidateCreateIDOpts rows pin the other-case id and the other-case bare prefix.
status: addressed
---
