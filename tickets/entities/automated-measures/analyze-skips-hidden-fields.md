---
id: analyze-skips-hidden-fields
type: automated-measure
title: Analysis reports no finding on a field hidden from the reader
description: 'TestACLAnalyze_PropertiesSkipsHiddenFields and TestAnalyzeProperties_SkipsHiddenFields run analysis as a role whose visible: hides a required field and assert that only the visible field is reported.'
kind: test
location: internal/dataentry/acl_analyze_test.go and internal/mcp/analyze_face_test.go
status: active
---

Fails when the property check validates a redacted row without dropping the
fields in its Redacted list.
