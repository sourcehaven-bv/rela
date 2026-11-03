---
id: RR-J7URXL
type: review-response
title: getErrorMessage default fallback stays English
finding: Callers that omit the fallback get 'Something went wrong' in every language.
severity: minor
resolution: 'Plan: the default fallback becomes t(''errors.generic'').'
status: addressed
---
