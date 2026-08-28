---
id: RR-Q40EAS
type: review-response
title: 'text-transform: uppercase dropped from THREE surfaces, not two, and never announced as intentional'
finding: |-
    All three deleted .properties-list blocks carried `text-transform: uppercase` on `dt` -- PropertyDisplay.vue:129, SectionEditForm.vue:321, and SidePanel.vue:313. The consolidated rule in properties-list.css does not.

    I had described this as 'two uppercase rules' in the implementation checklist and framed it as a welcome side effect. It was three, and I never recorded it as a deliberate decision anywhere an author or reviewer would see -- not in the ticket's acceptance criteria, not in docs/data-entry.md, not in frontend/CLAUDE.md.

    That makes it an unannounced visual change to every property label on the detail page, the section-edit form, AND the side panel. It reads as collateral damage from a layout refactor, which is not something a reviewer can approve by inspection.

    I still think sentence-case is the right call (SHOUTED labels on the detail page next to sentence-case labels in forms was one of the original inconsistencies). But it needs to be stated as a decision with its rationale, not discovered in a diff.
severity: significant
resolution: |-
    Fixed in 2ff8e0db. The removal stands -- sentence-case is right, and uppercase also mangles non-English labels, which matters for a metamodel that is deliberately language-neutral. What was wrong was leaving it implicit.

    It is now stated as a decision with its rationale in a comment at the rule in properties-list.css, and pinned by a test in propertiesListGrid.test.ts so it cannot be silently reverted.

    Also corrected the count in the implementation checklist: it was THREE surfaces (PropertyDisplay, SectionEditForm, SidePanel), not two as I had recorded. Undercounting a visual change in the evidence I asked a reviewer to trust is the more embarrassing half of this finding.
status: addressed
---
