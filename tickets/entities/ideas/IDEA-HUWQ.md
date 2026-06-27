---
id: IDEA-HUWQ
type: idea
title: 'Explicit permission catalog: named-verb model replacing today''s read/write type-lists'
description: 'Today''s ACL uses two verb-flavored type lists (read/write × entity types). New verbs being surfaced are unaddressable in this shape: per-script execution (UI buttons, MCP lua_run, CLI rela run), lua_eval carve-out, analyze:run, audit:read, read sub-verbs (list/count/relations/field), attachments. User proposes a Plone/Zope-style named-permission catalog with roles as bundles of permissions. Three-category model locked in: (1) action scripts use script.<name> per-permission, triggered by user; (2) schedules run as server-side scheduler identity, no per-script gate; (3) validations run as engine identity, no gate; (4) lua_eval needs its own script.eval-arbitrary permission.'
category: architecture
inspiration: 'Conversation on TKT-VQGN PR 939. User points to Zope/Plone explicit permission model. Verbs already declared needed: analyze, scripts, audit, plus read sub-verbs surfacing in TKT-VMD8.'
effort: large
value: valuable
notes: 'Recommended sequencing: hold until TKT-VMD8 lands (confirms read-verb shape under implementation pressure), then return with concrete verb list + dogfood-project migration scope. Most likely end-state: Option A (catalog as data, roles as bundles) + Option D (per-script permissions auto-register from script @permission headers). Migration via auto-compile legacy read:[ticket] → explicit grants. Open questions: catalog format (data file vs generated-from-code), scope syntax (entity-type, field-name, what else?), granularity ceiling (one analyze.run vs per-check), migration scope of TKT-9E57 field-scoped grants. References: DEC-RG878 (four-layer model — research builds on, doesn''t replace), FEAT-AESD4 (Authorization for data-entry and MCP), TKT-9E57 (predicate-backed _fields/_relations resolver, done).'
status: exploring
---
