---
id: RR-5H2H0
type: review-response
title: No handler-level test for JSON-string fallback with null
finding: 'TestExtractPropertiesAllowNil_StringJSONNullDeletes covers the helper, but no test sends ''properties'': ''{"foo": null}'' through handleUpdateEntity end-to-end. Some MCP clients pass tool args as JSON strings; a regression in the string fallback would slip past helper-only coverage.'
severity: minor
resolution: 'Added TestHandleUpdateEntity_JSONStringPropertiesNullDeletes — sends `properties: ''{"status": null}''` (as a JSON-encoded string) through handleUpdateEntity end-to-end and asserts the property is deleted from the stored entity.'
status: addressed
---
