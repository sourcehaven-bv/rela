---
id: RR-978KZI
type: review-response
title: Templated display leaks past git-crypt locked-title guard in mentions
finding: internal/dataentry/mentions.go used GetPrimaryProperty() (now "" for templates) to decide if a mention's title is locked. For a templated display_property like "{achternaam}" where achternaam is git-crypt-locked, lockedReasonFor matched only "" or InaccessibleFieldContent, so the mention was NOT flagged inaccessible — a regression vs the bare-name display_property which was correctly protected. The title could render a partial name from other readable placeholders.
severity: significant
resolution: 'Added EntityDef.DisplayProperties() returning all property names backing the display title (all placeholders for a template, the single primary otherwise). Rewrote mentions.go displayProperties()/lockedReasonFor() to flag the title inaccessible when ANY display property (or whole-content) is locked. Added regression tests in mentions_test.go: locked placeholder → inaccessible; locked non-placeholder → still readable; fully-readable template → renders both.'
status: addressed
---
