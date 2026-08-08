---
id: RR-DSAN9B
type: review-response
title: Step-level hiding has no clear_when_hidden semantics — and that is the case the motivating e2e test exercises
finding: |-
    Plan step 4 adds clear_when_hidden as a PER-FIELD key. But a whole STEP can hide, and that is exactly the AC5 case the fix must invert: in e2e/tests/wizard.spec.ts the `done` checkbox hides the entire Assignment step carrying `assignee`. FormStep (internal/dataentryconfig/config.go:153-159) has no such key and the plan never addresses step-granular hiding.

    So an implementer must invent: is it the union of the step's fields' settings? A step-level key? Does a field-level setting override a step-level one? The plan's motivating example does not typecheck against the plan's own config surface.

    Related (from the security reviewer, RR-O0KRI2 sibling): the key belongs on FormField and the unified FormFieldOrRelation ONLY — not on FormRelation, whose clearing is the separate pruneWizardHiddenRelations mechanism. Adding it there would imply behavior that does not exist.
severity: significant
resolution: |-
    Resolved by design decision: clear_when_hidden is PER-FIELD ONLY. No step-level key.

    Verified FormStep (internal/dataentryconfig/config.go:153-159) has exactly five fields — Title, Description, VisibleWhen, Fields, Relations — with no per-field behavior key of any kind. Adding a step-level clear_when_hidden would invent new config surface, which is out of scope for a bug fix on existing behavior.

    Rule: a step hiding is simply 'all of its fields hid', each honoring its own clear_when_hidden setting. This covers the AC5 case (the `done` checkbox hiding the whole Assignment step carrying `assignee`) with zero new config surface. Also confirmed the key belongs on FormField and the unified FormFieldOrRelation only — not FormRelation, whose clearing is the separate pruneWizardHiddenRelations mechanism.
status: addressed
---
