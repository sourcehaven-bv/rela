---
id: RR-PO49W3
type: review-response
title: Property cardinality is untested — a duplicated PERCENT-COMPLETE passes the whole suite
finding: |-
    VERIFIED BY MUTATION: emitting PERCENT-COMPLETE twice from RenderTodo passes the entire test suite with no failure.

    RFC 5545 §3.6.2 makes UID, DTSTAMP, SUMMARY, STATUS, DUE, COMPLETED, PRIORITY and PERCENT-COMPLETE all MUST-occur-at-most-once in a VTODO. The tests use containsLine (= countLine > 0) everywhere, so they assert presence and never cardinality. countLine already exists and is used for exactly this purpose in two places — it is simply never applied to properties.

    This matters because duplicate properties are a classic iCalendar serializer failure, and clients resolve them inconsistently (first-wins vs last-wins) — the "works in Reminders, wrong in Fantastical" class of bug.

    Note STATUS duplication IS currently caught, but only accidentally, by the injection test's countLine(..., "STATUS:NEEDS-ACTION") != 1 assertion. That is an incidental guard, not an intentional one.

    (Cardinality of the CURRENT output is correct — independently verified that every VTODO-level property appears exactly once, the three DESCRIPTIONs being one per VALARM plus the VTODO's own as RFC 5545 requires. The defect is the absence of a test, not a live bug.)

    Fix: one table-driven test asserting countLine(lines, prop) <= 1 for every at-most-once property across a matrix of Todo shapes.
severity: significant
resolution: |-
    Fixed in commit 5a0cac4e. Added TestRenderTodo_PropertyCardinality: a table of five Todo shapes, asserting every at-most-once VTODO property (UID, DTSTAMP, SUMMARY, STATUS, DUE, COMPLETED, PRIORITY, PERCENT-COMPLETE, URL) appears at most once. It tracks VALARM nesting depth so a VALARM's own DESCRIPTION is correctly excluded from the VTODO-level tally.

    VERIFIED BY RE-RUNNING THE MUTATION: emitting PERCENT-COMPLETE twice previously passed the entire suite; it now fails with "PERCENT-COMPLETE appears 2 times, RFC 5545 permits at most once".
status: addressed
---
