---
id: RR-6PUF93
type: review-response
title: ReplaceEntityFamily already rejects undeclared types
finding: 'convert.go:37 already returns ''%w: type %q is not declared'' wrapping store.ErrInvalidFamilyReplacement. The check runs before any file move, so AC3 already holds. entitymanager/conversion_flow.go:275 classifies on that sentinel. Do not replace it with the plain unknown-type error. Reuse the shared helper but keep the ErrInvalidFamilyReplacement wrap. Reword the scope and AC3 as ''keep and cover'', not ''add''.'
severity: minor
resolution: 'Plan updated: ReplaceEntityFamily keeps its existing check and the store.ErrInvalidFamilyReplacement wrap; AC3 pins it.'
status: addressed
---
