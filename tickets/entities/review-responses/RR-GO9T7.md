---
id: RR-GO9T7
type: review-response
title: 'Test plan gaps: case-sensitivity, includes-merge'
finding: 'Plan''s test list lacks: (a) case-sensitivity (display_property: TITEL vs property: titel — should fail), (b) includes-merge (parent metamodel includes child where child entity has display_property — validation must run on merged result), (c) MCP exposure (passes through automatically; no test needed).'
severity: minor
resolution: 'Add to test list: Load_displayPropertyCaseSensitive (asserts case-mismatched lookup fails), Load_displayPropertyAcrossIncludes (parent + child file with display_property on child entity, both load + validate). MCP roundtrip already covered by existing integration tests.'
status: addressed
---
