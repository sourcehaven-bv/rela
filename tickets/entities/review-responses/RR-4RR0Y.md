---
id: RR-4RR0Y
type: review-response
title: Store interface still leaks markdown.Document
finding: The repository.Store interface has LoadEntityTemplate, LoadEntityTemplateVariant, and LoadRelationTemplate methods returning *markdown.Document. The plan addresses DiscoverEntityTemplates (EntityTemplate move) but ignores these three methods. Any consumer touching templates through Store will still need the markdown import.
severity: critical
resolution: 'Plan updated: markdown.Document will also move to model as a generic Document type (it''s just map[string]interface{} + string). The Store interface methods will return *model.Document instead. This is the same pattern as EntityTemplate — pure data, no markdown-specific behavior.'
status: addressed
---
