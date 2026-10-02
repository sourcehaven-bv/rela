---
id: FEAT-XT3SOP
type: feature
title: 'Spaces: named entry points onto one graph'
summary: A space is a named navigation, home page and create menu over the shared graph (CRM, ISMS, Projects), entered through a switcher in the sidebar header.
description: 'One rela instance, one schema, one graph. A space changes only what the user sees: its navigation, home page and Create menu. Relations, ACL and search stay shared, so content keeps cross-linking across spaces. Distinct from RES-S8CH9C (several separate projects in one server) and from content-state worlds (which face of an entity a reader sees).'
priority: medium
status: proposed
---

## Summary

A **space** is a named entry point onto the one graph a rela instance serves:
CRM, ISMS, Projects. It declares its own navigation, home page and Create menu,
and reuses the lists, views and forms that already exist, by id. Schema, data,
relations, ACL and search stay shared, so a CRM contact can still link to an
ISMS risk.

The user enters a space through a switcher at the top of the sidebar (Atlas
design, "Atlas Projects ▾").

## What a space is not

- **Not a separate project.** RES-S8CH9C researched several projects with
separate graphs in one server. A space is one project with several entry points;
nothing is partitioned.
- **Not a security boundary.** Data access stays with the existing type and
row ACL. Config is not a secret (CLAUDE.md, docs/acl-security.md).
- **Not a world.** A content-state world picks which face of an entity a reader
sees. A space picks which screens. They compose freely.

## Name

"Space", not "app": rela already has custom apps (sandboxed, listed under "Apps"
in the sidebar).
