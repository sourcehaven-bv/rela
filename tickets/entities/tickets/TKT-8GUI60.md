---
id: TKT-8GUI60
type: ticket
title: 'Replace emoji with an SVG icon set; add icon: to kanban columns and swimlanes'
kind: enhancement
priority: medium
effort: m
status: review
---

## Description

**PR 3 of 3** splitting TKT-5V8704 (see FEAT-OJ8L0H). Shares no files with the
layout PR, so it can land independently of PR 2.

Emoji cannot take `currentColor`, ignore the theme toggle, render differently
per OS, and sit on an inconsistent baseline — all while sitting directly beside
real SVG icons (search, trash, chevrons).

### Icon library selection

| Option | Icons | License | Notes |
|---|---|---|---|
| **Lucide** (`lucide-vue-next` 1.0.0) | 1,500+ | ISC | Feather fork, actively maintained, one outline path per icon. Official Vue 3 package. |
| Heroicons (`@heroicons/vue` 2.2.0) | ~292 | MIT | Smallest set — risk of gaps. |
| Phosphor | 7,700+ | MIT | Most variety/weights; larger surface than needed. |
| font-awesome 4.7 (already a dep) | — | — | **Rejected**: present only because EasyMDE's toolbar requires it (`MarkdownEditor.vue:10`, `relaEditor.ts:24`). v4 is an icon *font* — no tree-shaking, ships the whole webfont, no per-path `currentColor`. Reusing it entrenches a legacy dep. |

**Recommended: Lucide** — tree-shakeable named imports, self-hosted (no runtime
fetch, mandatory since the SPA is embedded in a Go binary), `currentColor` by
default so it themes for free.

### Sidebar (from RR-09N4MN)

`Sidebar.vue:101` already has the right shape — a `getIconEmoji` allowlist
switch over `list`/`kanban`/`dashboard` with a default fallback. The work is
changing the return type from string to component, **not** designing a new
validation surface.

Do not miss the emoji **outside** that map: `Sidebar.vue` lines 161, 166, 236,
249, 257 (search, analysis, apps, settings, and the theme-toggle sun/moon).

### Kanban (from RR-GWVGDX — the critical finding)

Kanban has **no icon field at all**. The glyphs are literal characters inside
user-authored `label:` strings
(`prototypes/data-entry/project/data-entry.yaml:551-556`):

```yaml
- value: open
  label: "📥 To Do"
```

The SPA must **never** parse emoji out of user label text — that is a lossy
heuristic over user data and silently rewrites what an author typed. Instead:

- `KanbanColumn` (`config.go:396`) gains an optional `Icon string`.
- `KanbanSwimlane` (`config.go:402`) gains it too — identical `Value`/`Label`
shape, labels rendered the same way, so icon-ing only one leaves the model
asymmetric.
- The prototype YAML migrates its emoji out of `label:` into `icon:`.

Corrects an earlier observation: the "missing space" in `📥To Do` was **not** a
bug. The label is authored with a space; it merely reads tight. No SPA fault.

### Validation (from RR-IUMZV8)

Icon names are resolved via a **static allowlist**, never dynamic component
resolution from a config string. An unknown name is a **load-time config error**
(`validateKanbans` already iterates columns with indexed messages), consistent
with the strict `ValidateConfig` convention — plus a frontend default fallback
so a stale config renders rather than throws.

## Acceptance criteria

1. An SVG icon set is added with the licensing/bundling rationale recorded; it
is self-hosted with no runtime network fetch for icon assets.
2. Sidebar emoji are replaced with SVG icons, **including** the five hardcoded
glyphs outside `getIconEmoji` (lines 161, 166, 236, 249, 257).
3. Icons inherit `currentColor` and follow the theme toggle in both themes.
4. `KanbanColumn` and `KanbanSwimlane` accept an optional `icon:`; the SPA
renders it beside the label and never parses emoji out of `label:` text.
5. The prototype `data-entry.yaml` is migrated from emoji-in-label to `icon:`.
6. An unknown icon name is a load-time config error with an indexed message;
the frontend independently falls back to a default icon rather than throwing.
7. A `label:` that still contains an emoji renders verbatim — never stripped.
8. No regression in frontend unit tests, Go tests, or the e2e suite.

## Out of scope

- Tokens (PR 1) and the span/layout work (PR 2).
- Label humanization — separate PR entirely.
