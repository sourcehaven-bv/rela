---
finding: When LoadEntityTemplateVariant returns (nil, nil) for non-existent variant, the entity is created without template defaults and NO error. AC#4 states errors should be reported for missing templates.
id: RR-wjy2
resolution: 'Added check in createEntityNoAutomation: if templateVariant is non-empty but template is nil, return error. TestCreateEntity_AutomationWithMissingTemplate verifies this.'
severity: significant
status: addressed
title: Missing template variant not surfaced as error
type: review-response
---
