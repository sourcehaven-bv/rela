---
id: TKT-GNKR5H
type: ticket
title: 'Spaces v1: spaces: in data-entry.yaml, /s/<space>/ URL prefix, sidebar switcher'
kind: enhancement
priority: medium
effort: l
status: backlog
---

## Description

First slice of FEAT-XT3SOP. One graph, several named entry points.

### Config

```yaml
spaces:
  projects:
    label: "Atlas Projects"
    icon: folder
    home: dashboard_projects        # a dashboard, list or view id
    create: [project, taak]         # the Create menu in this space
    navigation: [...]               # same shape as today's navigation:
  isms:
    label: "ISO 27001"
    permission: isms:use            # hides the space from the switcher
    home: isms_dashboard
    create: [risk, control, incident]
    navigation: [...]
```

Today's top-level `navigation:` stays valid and acts as the implicit default
space. A config with no `spaces:` behaves exactly as now, and with fewer than
two spaces the switcher is not rendered.

### Decisions (agreed 2026-09-27)

- **Name:** "space". "App" is taken by custom apps.
- **URL:** the space is a path prefix, `/s/<space>/list/...`, so links keep
their context and back works. The prefix is SPA-only: the server's API routes do
not change.
- **An entity page is valid in every space.** Following a link or a search
result keeps the user in the current space and renders the entity there.
Resolving a "home" space per entity type is a possible follow-up, not v1.
- **Search** stays global.
- **Create** is bound to the space: the Create menu offers the space's
`create:` types.
- **ACL:** a space is not a security boundary. `permission:` exists for UX
only, to keep a space that makes no sense for a role (CRM for an auditor) out of
their switcher. It does not protect data: the lists and entities behind it stay
reachable by URL under the normal ACL. It reuses the navigation `permission:`
mechanism (TKT-TXDK8U). Per the CLAUDE.md rule on config, the docs must say this
gate is for tidiness, not concealment.
- **Switcher UI** is composed here from library primitives; rela-components
  declined a dedicated space switcher because a space is a rela concept
  (library commit 513f8a9 added the missing pieces). Put `RlMenu` with
  `align="start"` in RlSidebar's `#switcher` slot. Its trigger is
  `RlWorkspaceSwitcher`, with `:menu="spaces.length > 1"`, so one space
  renders as a plain label. Use one `RlMenuItem` per space, with `href` (a real
  link) and `current`. Pass the current space's icon through the `#logo` slot,
  since that is all the collapsed rail shows.

### Scope

- `dataentryconfig`: the `spaces:` type, validation (ids, unknown
list/view/dashboard/form references, `create:` types exist, home resolves,
permission warnings like TKT-E5EM3N), the implicit default space.
- Sidebar API: the spaces the principal may enter plus the navigation of the
requested space. Nav status keys become per space.
- SPA: `/s/:space/` route prefix, a current-space store derived from the
route, the switcher, the Create menu from `create:`, redirects from unprefixed
URLs to the default space.
- Docs: data-entry.md section, acl-security.md note on the UX-only gate.

## Out of scope

- Splitting schema.yaml or data-entry.yaml into per-space files, and
installable space packages. Type-name prefixes would come with that.
- A per-entity default space for links and search results.
- Several projects in one server (RES-S8CH9C).

## Acceptance criteria

- A config without `spaces:` renders and routes exactly as today.
- With two spaces, the switcher lists both, switching changes the sidebar,
home and Create menu, and the URL carries `/s/<space>/`.
- A space with a `permission:` the principal lacks is absent from their
switcher, and its lists stay reachable under the normal ACL.
- Opening an entity from search or a relation link keeps the current space.
- Invalid references in `spaces:` fail config load with a message naming the
space and the key.
