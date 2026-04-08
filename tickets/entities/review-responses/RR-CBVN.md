---
id: RR-CBVN
type: review-response
title: parseFilterQueryParams missing property-name validation
finding: 'The regex /^filter\[([^\]]+)\](?:\[([^\]]+)\])?(\[\])?$/ accepts any non-] character as a property name, including __proto__, brackets, whitespace, and Unicode. Setting result[''__proto__''] = filterValue is a prototype-pollution risk on older engines. Even on modern V8 it''s surprising behavior. Fix: add an allowlist regex /^[a-zA-Z_][a-zA-Z0-9_]*$/ on the property name and skip entries that don''t match.'
severity: significant
resolution: filters.ts adds PROPERTY_NAME_RE = /^[a-zA-Z_][a-zA-Z0-9_]*$/ as an allowlist. parseFilterQueryParams drops any key whose property fails the check. Prevents prototype pollution via __proto__ and rejects names with brackets, spaces, hyphens, or leading digits. Regression test covers all rejected forms plus a valid control case.
status: addressed
---
