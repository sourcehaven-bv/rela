---
id: RR-D5F6R
type: review-response
title: reqEntity helper unused title parameter
finding: internal/dataentry/document_script_test.go:128. All callers pass "a req" and no test reads result.Properties["title"] back. Per CLAUDE.md fluent-builder guidance, auto-generate in the helper or drop the parameter.
severity: nit
resolution: reqEntity() helper now takes no arguments (hardcoded title 'a req' internally). All callers updated.
status: addressed
---

From post-impl cranky review.

Fix: drop the title parameter, hardcode "a req" inside the helper (or let the
entity manager pick a default). Small cleanup.
