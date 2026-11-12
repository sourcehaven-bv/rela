---
id: RR-M4O0XGC
type: review-response
title: req asserts no status code
finding: _batch and _reconcile decode strictly. A wire rename turns the passing requests into 400s silently, and the weekly run reports the handlers untraced a week later.
severity: minor
resolution: req now takes EXPECT and dies on a mismatch. Every request this ticket adds sets it.
status: addressed
---
