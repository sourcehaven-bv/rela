---
id: RR-field-name-validation
type: review-response
title: Field name validation is underspecified
finding: |
  The validation schema says field `name` must be "Non-empty, unique within form" but doesn't specify allowed characters. Field names are used as keys in the returned event.data table and potentially as form element identifiers.
  
  Missing specification:
  - What characters are allowed? (alphanumeric only? underscores? hyphens?)
  - What about reserved names? (e.g., "type", "action", "__index")
  - What about unicode in field names?
  - Maximum length?
  
  Recommendation: Use allowlist validation - alphanumeric plus underscore, starting with letter, max 64 chars. This matches common identifier rules and avoids Lua reserved word conflicts.
severity: minor
status: addressed
resolution: Added explicit identifier format to validation schema - `[a-zA-Z][a-zA-Z0-9_]*`, max 64 chars, unique within form.
---
