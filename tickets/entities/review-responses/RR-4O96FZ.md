---
id: RR-4O96FZ
type: review-response
title: '`render` on shared SectionField wire type reaches side panels (not inert — a renderer gap)'
finding: Review flagged that `v1.SectionField` is embedded in four response structs (responses.go:318 SidePanelSection, :348 SidePanelEntity, :477 ViewSection, :513 ViewEntity), so adding `Render` emits the key on side-panel surfaces that have no inline edit, and characterised it as leaking a meaningless field into a shared DTO.
severity: minor
resolution: 'Accepted as a documentation item, rejected as a design defect. Side panels reuse ViewSection configs verbatim (`views_handler.go:130`: `Sections: form.SidePanel.Sections`), so an operator writing `render:` there is expressing real intent — SidePanel.vue simply has no inline-edit renderer yet. The key is unhonoured-by-that-renderer, not meaningless. Keeping `Render` on the shared type preserves the option of side-panel inline edit; splitting the type would structurally exclude it. Ship on the shared type; note the side-panel renderer gap in the docs.'
status: addressed
---

The review's framing conflated "a renderer does not honour this key yet" with
"this key has no meaning on this surface". Only the first is true.

`SidePanel.vue` consumes `SidePanelSection[]` and renders fields read-only; it
never mounts `SectionEditForm`. That is a current limitation of that component,
not a property of the data. Since side-panel sections *are* `ViewSection`
configs, a future ticket enabling inline edit there would want exactly this key.

Decision: single `Render` field on `v1.SectionField`. Document that the
side-panel renderer ignores it today.
