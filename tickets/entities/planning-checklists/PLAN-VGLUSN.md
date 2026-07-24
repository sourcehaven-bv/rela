---
id: PLAN-VGLUSN
type: planning-checklist
title: 'Planning: Surface metamodel doc-fields in data-entry help: per-entity values+lifecycle, plus a global help icon showing the app description'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/scope/AC clear (see ticket)
- [x] Depends on TKT-0YBFT8 (same branch)

## Approach

**Part A — per-entity help (Go, handlers.go, HTML endpoint):**
- Extend `handleEntityHelp`: for each enum/state-machine property, resolve its
`CustomType` (`s.Meta.Types[prop.Type]`, or inline `prop.Values`/`Labels`).
- New struct(s): `ValueHelp{Value, Label, Description}` and
`TransitionHelp{From, To, Label, Help}`, gathered per property that is an enum.
- Extend `renderHelpContent` with:
  - **Values** section per enum property: a `.help-table` (Value | Description),
using label when set, description via `simpleMarkdownToHTML`. Omit if the type
has no values.
  - **Lifecycle** section per state-machine property (type has `Transitions`):
a `.help-table` (Move | From→To | When to use) with label + Help.
- Sections omitted when empty (no noise). Mirrors the existing Properties/
Relations rendering exactly; HelpModal.vue CSS unchanged.

**Part B — global app description (config + status bar):**
- Go: `handleV1Config` builds `v1.Config`; add `Description` to `v1.AppConfig`
and populate from `s.Meta.Description`. (Distinct from `toV1CustomType` — pin
test untouched.)
- TS: add `description?` to the config/app TS type.
- SPA: add a help/`?` button to the bottom status bar next to Settings + theme
toggle (StatusBar.vue or the layout that renders them), opening an About overlay
bound to the config description. Reuse HelpModal.vue's overlay styling (or a
thin About modal). Button hidden when description is empty.

**Files:**
- `internal/dataentry/handlers.go` (values + lifecycle sections + structs)
- `internal/dataentry/handlers_test.go` (or new) — help-render tests
- `internal/dataentry/api_v1.go` (`handleV1Config` → App.Description) +
`internal/apiwire/v1/responses.go` (AppConfig.Description)
- `frontend/src/types/*` (config type), a StatusBar/layout component, an About
overlay, + a small component test

## Test Plan

- [x] Go: help endpoint renders a Values section (value+desc) for an enum prop;
a Lifecycle section for a machine prop; omits both for a plain field. Config
carries `description`.
- [x] SPA: status-bar help button renders; opens the About overlay with the
description; hidden when absent.
- [x] Edge: value with no description → value/label only; type with no
transitions → no Lifecycle section; empty Metamodel.Description → no button.
- [x] Manual: demo against the prototype project.

## Security

- [x] Help HTML is built with `htmltemplate.HTMLEscapeString` for values/labels
and `simpleMarkdownToHTML` for prose (existing goldmark path, DOMPurify on the
client) — same trust model as the existing description rendering. Config
description is JSON (escaped by the encoder). No new injection surface.

## Risk

- [x] Small; mirrors established patterns. Main watch-item: don't accidentally
wire the doc-fields to `toV1CustomType` (keep the pin test green) — Part A is
the HTML endpoint, Part B is config, neither touches it.
- Effort: s.

## Docs

- [x] N/A user-facing doc change (this is UI surfacing of already-documented
fields). Docs-checklist on entering implementation, likely mostly N/A.

## Design Review

- [x] ~~/design-review~~ — mechanical, follows the existing help-render +
config patterns; no novel design surface.
