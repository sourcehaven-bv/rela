---
id: RR-H10L
type: review-response
title: Test had dead _ = got + inconsistent header-set paths
finding: TestHeaderPrincipalResolver_Sanitizes/control_chars_replaced built a runResolver call, discarded its result with _ = got, then constructed a second request manually because http.Header.Set strips \n. Dead code; dual API.
severity: significant
resolution: 'Extracted resolveHeaderRaw(t, name, value) helper that writes the canonical-form req.Header[name] = []string{value} map directly. The two control-char subtests + the new control-only-regression subtest all use it. Dead _ = got removed. File: internal/dataentry/principal_test.go:145-194.'
status: addressed
---
