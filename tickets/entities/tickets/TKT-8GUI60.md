---
id: TKT-8GUI60
type: ticket
title: 'Replace emoji with an SVG icon set; add icon: to navigation, kanban columns and swimlanes'
kind: enhancement
priority: medium
effort: m
status: done
---

## Description

**PR 3 of 3** splitting TKT-5V8704 (see FEAT-OJ8L0H).

Emoji cannot take `currentColor`, ignore the theme toggle, render differently
per OS, and sit on an inconsistent baseline — all while sitting directly beside
real SVG icons (search, trash, chevrons).

### Icon library selection

| Option | Icons | License | Notes |
|---|---|---|---|
| **Lucide** (`lucide-vue-next` 1.0.0) | 1,500+ | ISC | Feather fork, actively maintained, one outline path per icon. Official Vue 3 package. |
| Heroicons (`@heroicons/vue` 2.2.0) | ~292 | MIT | Smallest set — risk of gaps. |
| Phosphor | 7,700+ | MIT | Most variety/weights; larger surface than needed. |
| font-awesome 4.7 (already a dep) | — | — | **Rejected**: present only because EasyMDE's toolbar requires it. v4 is an icon *font* — no tree-shaking, ships the whole webfont, no per-path `currentColor`. Reusing it entrenches a legacy dep. |

**Chosen: Lucide** — tree-shakeable named imports, self-hosted (no runtime
fetch, mandatory since the SPA is embedded in a Go binary), `currentColor` by
default so it themes for free.

### Sidebar glyphs (from RR-09N4MN)

`Sidebar.vue` had a `getIconEmoji` allowlist switch over
`list`/`kanban`/`dashboard` with a default fallback — the right shape already.
The work is changing the return type from string to component.

Do not miss the emoji **outside** that map: search, analysis, apps, settings,
and the theme-toggle sun/moon. Nine glyphs total.

### Kanban (from RR-GWVGDX — the critical finding)

Kanban had **no icon field at all**. The glyphs were literal characters inside
user-authored `label:` strings:

```yaml
- value: open
  label: "📥 To Do"
```

The SPA must **never** parse emoji out of user label text — that is a lossy
heuristic over user data and silently rewrites what an author typed. Instead
`KanbanColumn` and `KanbanSwimlane` each gain an optional `Icon string` (same
`Value`/`Label` shape, rendered the same way, so supporting one and not the
other would be an arbitrary asymmetry). The prototype YAML migrates its emoji
out of `label:` into `icon:`.

Corrects an earlier observation: the "missing space" in `📥To Do` was **not** a
bug. The label is authored with a space; it merely reads tight.

### Navigation entries

**Added after the sidebar swap, on the same principle.** The nav icon was
derived from the entry KIND in `navEntryToSidebarItem` — every `list:` entry the
same glyph, every `kanban:` the same one. A sidebar with three ticket lists
therefore read as three identical rows distinguishable only by their labels; the
icons carried no information.

`NavigationEntry` gains an optional `Icon string` that overrides the derived
default. Applied **after** the kind switch so it covers every branch — including
`action:` entries, which derive no icon at all and were otherwise the one kind
that could never have one.

A **group** is rejected rather than silently ignored: it renders as a bare
section title with nowhere to put an icon. Same reasoning and message shape as
the existing group-`permission` rejection.

The frontend needed no change: `navIcon(item.icon)` already resolved through the
registry with a safe fallback.

### Validation

Icon names resolve via a **static allowlist**, never dynamic component
resolution from a config string. An unknown name is a **load-time config error**
with an indexed message, consistent with the strict `ValidateConfig` convention
— plus a frontend default fallback so a stale config renders rather than throws.

`ValidIconNames` (Go) mirrors the SPA registry, pinned by a test that reads the
real `icons.ts` and fails on drift in **either** direction.

## Acceptance criteria

1. An SVG icon set is added with the licensing/bundling rationale recorded; it
is self-hosted with no runtime network fetch for icon assets.
2. All nine sidebar emoji are replaced with SVG icons, **including** the five
outside the old `getIconEmoji` switch.
3. Icons inherit `currentColor` and follow the theme in both modes.
4. `KanbanColumn` and `KanbanSwimlane` accept an optional `icon:`; the SPA
renders it beside the label and never parses emoji out of `label:` text.
5. `NavigationEntry` accepts an optional `icon:` that overrides the
kind-derived default, including for `action:` entries.
6. An icon on a navigation **group** is a config error, not silently dropped.
7. The prototype `data-entry.yaml` is migrated from emoji-in-label to `icon:`,
and demonstrates per-item nav icons.
8. An unknown icon name is a load-time config error with an indexed message;
the frontend independently falls back to a default icon rather than throwing.
9. A `label:` that still contains an emoji renders verbatim — never stripped.
10. The Go allowlist and the SPA registry are pinned to each other by a test
that fails on drift in either direction.
11. No regression in frontend unit tests, Go tests, or the e2e suite.

## Out of scope

- Tokens (PR 1) and the span/layout work (PR 2).
- Label humanization — separate PR entirely.
- Emoji in other components (EntityList, CommandModal, InaccessibleField,
StatusBar) — separate surfaces this ticket does not cover.
