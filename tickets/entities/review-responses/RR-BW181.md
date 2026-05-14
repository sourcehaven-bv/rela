---
id: RR-BW181
type: review-response
title: Warning code mapping too coarse — type_mismatch collapses semantically distinct conditions
finding: 'Mapping puts everything tagged InvalidType under property_type_mismatch. But that error fires for ''must be a string'', ''must be an integer'', ''must be RRULE'', etc. — all distinct user-facing messages a UI may want to render differently. CLAUDE.md mandates warning codes match analyze_validations codes. Plan never checks what analyze emits. Recommendation: run analyze_validations with each violation class first, copy codes verbatim. Split property_type_mismatch (wrong primitive) from property_value_invalid (right type, value rejected). Document full table. From design-review F3.'
severity: significant
resolution: 'Verified during plan rewrite: no existing analyze code vocabulary for built-in property validations (validator.Violation has user-defined RuleName; built-in validation is write-time only). The codes this ticket defines ARE the canonical codes. Mapping table is the three-class minimum: property_type_mismatch (wrong primitive), property_value_invalid (right type, value rejected), required_property_unset. Documented in Research section.'
status: addressed
---
