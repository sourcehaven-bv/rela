---
id: DOCS-XJVZBS
type: docs-checklist
title: 'Documentation: Inline entity creation from relation fields (TKT-OMUD56)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported/public symbols documented — `SidebarResponse.InlineCreate`,
`viewsHandler.inlineCreateForms`, `useInlineCreate`, `provideInlineCreateDepth`,
`FormWizardOptions.syncUrl`, `DynamicForm`'s `embedded` prop and its
`defineExpose` handle all carry doc comments explaining **why**, not just what.
- [x] Non-obvious decisions explained in-place: why the sidebar (not `_schema`
or `_config`) carries the map; why the server sends the resolved form id rather
than letting the client mirror natsort ordering; why the ACL scope is resolved
once; why the nested form mounts under `v-if`; why the emit names are
namespaced; why `selectTarget` clears the notice before `handleInlineCreated`
sets it.

## Project Documentation

- [x] `docs/data-entry.md` — rewrote the "Inline creation" section: the derived
rule (permission ∧ a registered form), that a small dedicated form yields a
small modal, the edit-form fallback, the `visible_when` scope caveat, the
one-level nesting cap, and a **Changed** note that `allow_create` /
`create_form` are no longer read.
- [x] Removed the two knobs from the relation-field reference table and from
the worked example.
- [x] `docs-project/entities/guides/GUIDE-data-entry.md` — the same edits
applied to the mirrored guide, so the two do not drift.
- [x] ~~`docs/metamodel.md`~~ (N/A: no metamodel change)
- [x] ~~`docs/cli-reference.md`~~ (N/A: no command change)
- [x] ~~`CLAUDE.md`~~ (N/A: follows the existing affordance / consumer-side
patterns; no new architectural rule)

## External Documentation

- [x] Operator-facing migration note included in `docs/data-entry.md` as the
**Changed** callout: leaving the keys in YAML is harmless (unknown nested keys
are silently ignored), but a `+ New` button will disappear where the target type
has no registered create form.
- [x] Prototype configs updated so they no longer document behaviour they no
longer have (`prototypes/data-entry/project/` and `.../catalog/`).
- [x] ~~Release notes / blog~~ (N/A: no CHANGELOG file in this repo; the docs
callout is the operator-facing record)
