---
id: FEAT-8cwr
status: implemented
title: Help modal for entity documentation in data-entry
type: feature
---

## Summary
Add optional `description` fields to metamodel entities, properties, and relations. Surface these in data-entry via a help icon (?) that opens a modal showing documentation.

## Features
- Entity type descriptions explaining when/how to use each type
- Property descriptions with type info and required indicator  
- Relation descriptions showing cardinality and required status
- Markdown rendering for all descriptions

## Implementation
- Added `description` field to `EntityDef` in metamodel (properties/relations already had it)
- New `/api/help/{entityType}` endpoint returns rendered HTML
- Help icon in form header opens modal via JavaScript
- Template-based rendering with reusable partials
