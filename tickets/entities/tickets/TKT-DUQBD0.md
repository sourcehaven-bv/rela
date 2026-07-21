---
id: TKT-DUQBD0
type: ticket
title: 'Surface metamodel doc-fields in data-entry help: per-entity values + lifecycle (mermaid), plus a global About help'
kind: enhancement
priority: medium
effort: s
status: done
---

<!-- @managed: claude-workflow v1 -->

## Problem

Phase 1a (TKT-0YBFT8) added three metamodel doc-fields (`Metamodel.Description`,
`CustomType.Descriptions` per-value, `TransitionDef.Help`), but nothing renders
them yet. The data-entry help modal (FEAT-8cwr, `/api/help/{type}`, server HTML)
shows entity/property/relation descriptions but has no enum-values or lifecycle
section, and there's no place for the top-level app description. This surfaces
them in the existing help UI now, without the offline generator (phase 2).

## Scope

**In:**
1. **Values section** per enum/state-machine property: value + label +
`CustomType.Descriptions[value]`.
2. **Lifecycle section** per state-machine property: a **mermaid
`stateDiagram-v2` diagram** (one per machine field) — `[*] --> initial` + `from
--> to: move` per transition — PLUS a transitions table (move, from→to, `Help`).
Mermaid is bundled in the SPA; the help modal now runs `renderMermaidDiagrams`
over the injected HTML (mirrors DocumentView), so the `<pre class="mermaid">`
blocks the server emits render as SVG.
3. Server-side in `handleEntityHelp`/`renderHelpContent`
(`internal/dataentry/handlers.go`); sections omitted when empty.
4. **Global About help** in the bottom status bar (next to Settings + theme
toggle): a `ⓘ About` button opening an overlay with the deployment description
(`v1.AppConfig.Description`, which now falls back to `Metamodel.Description`
when the data-entry.yaml `app.description` is empty). Button hidden when there
is no description.

**Out:**
- Per-value / transition tooltips on the JSON `_schema`/`_transitions` wire
(in-context) — deferred; a separate contract change. The help endpoint (1,2)
does NOT touch `toV1CustomType` or its pin test.
- The offline `rela docs` generator (phase 2).

## Acceptance criteria

1. Entity help with an enum property → Values section (value + description; no
description → value/label only).
2. State-machine property → Lifecycle section with a mermaid state diagram
(rendered as SVG in the modal) + a transitions table (label + Help). One diagram
per distinct state-machine field.
3. Empty → sections omitted (no noise on plain fields).
4. `toV1CustomType` + its pin test untouched.
5. Global `ⓘ About` button in the status bar shows the app/deployment
description; hidden when empty.
6. Go tests cover the values + lifecycle (incl. mermaid block) render and the
config-description fallback.
7. Verified live in the demo (prototype project): descriptions/help/diagram
render in the modal; About shows the description.

## References

- Depends on TKT-0YBFT8 (fields). Same branch/PR bundle.
- Feature: FEAT-G4VO53; FEAT-8cwr (help modal).
- `internal/dataentry/handlers.go`; `handleV1Config`/`v1.AppConfig`;
`HelpModal.vue`; `StatusBar.vue`; `utils/markdown.ts` (renderMermaidDiagrams).
