---
id: RR-FVDPRD
type: review-response
title: side_panel sections bypass all config validation, so span is unchecked there
finding: |-
    validate.go contains ZERO references to SidePanel (verified: `grep -c SidePanel internal/dataentryconfig/validate.go` = 0). validateViews walks only cfg.Views; validateForms never descends into form.SidePanel.

    But SidePanelConfig.Sections (config.go:163) is []ViewSection -- the SAME struct carrying Fields []ViewSectionField, which now has Span. And it is not inert: executeSidePanel (sections.go:337) builds a synthetic ViewConfig from panel.Sections and calls the same buildSections, so buildSectionFieldData carries the span through to v1.SectionField and onto the wire.

    The reviewer demonstrated a side panel with `span: 999` returning zero errors while still propagating to the frontend.

    This makes validateSpan's own doc comment false for a real config path: it claims spans are rejected loudly at load, and for side_panel fields they are not. The frontend clamp catches the render, so nothing breaks visually -- but the author gets exactly the silent-ignore behaviour the strict validation was written to prevent.

    Broader context: property names in side_panel sections are not validated either, so this is a pre-existing structural gap rather than something this PR introduced. Span is simply the first key to make it visible.
severity: critical
resolution: |-
    Fixed in 2ff8e0db. Added validateSidePanelSpans, called from validateForms for every form's SidePanel, with an indexed message (`form %q: side_panel section[%d] field[%d]`). Pinned by TestSidePanelSpansAreValidated, which asserts a side-panel span of 999 is now rejected.

    Deliberately scoped to span. Side-panel property names have never been validated either, and fixing that is a wider behavior change -- it could start rejecting configs that load today -- so it belongs in its own ticket rather than riding along with a layout PR. The comment in validate.go says so, so the narrow scope reads as a decision rather than an oversight.
status: addressed
---
