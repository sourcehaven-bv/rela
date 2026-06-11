---
id: RR-FB2C
type: review-response
title: 'Round 2 NEW-3: verdict-flip toast conflated with server 403 — spurious refetch'
finding: |
  Routing the verdict-flip toast through `onError({ status: 403 })` causes EntityDetail's handleSectionEditError to fire `loadView()` again — one extra GET per flip. Not infinite, but semantically wrong: the verdict-flip notification is a client-side reconciliation announcement, not a server rejection.
severity: significant
status: addressed
resolution: |
  Separate path: add an `onVerdictFlip?: (prop: string, label: string) => void` callback prop to SectionEditForm. Plan AC 6 amended: the verdict-flip watcher calls `onVerdictFlip(prop, field.label)`, not `onError(..., { status: 403 })`. EntityDetail wires `onVerdictFlip` to a toast-only handler — no loadView retrigger. server 403s continue to flow through `onError` and trigger loadView as before.

  This also makes the verdict-flip path unit-testable in isolation from server-error handling. Both callbacks distinct, distinct test cases.
---
