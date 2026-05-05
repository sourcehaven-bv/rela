---
id: RR-8N1CZ
type: review-response
title: JSON-string variant of properties not addressed
finding: 'extractProperties at internal/mcp/tools_helpers.go:59-63 has a fallback: if properties arrives as a JSON-encoded STRING (some MCP clients pass it that way), it json.Unmarshals into map[string]interface{}. The plan''s new helper must handle the same string fallback identically — otherwise clients that pass {"properties": "{\"foo\":null}"} as a string see a regression where delete-via-null silently breaks for them. Add this case to the test plan: a request whose ''properties'' arg is a JSON-encoded STRING containing a null value, and verify it deletes the property.'
severity: significant
resolution: 'Plan updated: extractPropertiesAllowNil reuses the same JSON-string fallback as extractProperties (refactor: extract a shared parsePropertiesArg helper, then have the two variants differ only in whether they filter nil). Test added: TestExtractPropertiesAllowNil_StringJSONNullDeletes asserts that a JSON-encoded string `"{\"foo\": null}"` results in a map containing `foo: nil`.'
status: addressed
---
