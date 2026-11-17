---
id: RR-59TJIZT
type: review-response
title: 'Design: placeholders are expanded twice'
finding: Interpolate expands {{old.*}} and then rescans the result for {{new.*}}; a stored value holding {{new.secret}} makes any trigger using {{old.p}} write a hidden field. The checker cannot see it.
severity: critical
resolution: Interpolate now expands every placeholder in one pass and never rescans substituted text. Test in template_test.go.
status: addressed
---
