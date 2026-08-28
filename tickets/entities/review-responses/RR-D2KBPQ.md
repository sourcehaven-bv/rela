---
id: RR-D2KBPQ
type: review-response
title: Component/slice mismatch silently renders an empty calendar; the branch is duplicated three times
finding: |-
    Feed{Component: ComponentTodo, Events: [...]} renders an EMPTY VCALENDAR — silently. No error, no warning; the events vanish. The inverse (ComponentEvent with Todos populated) does the same.

    The branch is re-derived in three places: RenderCollection (ical.go:60), CollectionTag (ical.go:218), RenderJSON (json.go:78). The CalDAV adapter in TKT-MF1CWZ will be the fourth consumer and gets to reinvent it, possibly with a different fallback.

    The zero-value claim in the ticket IS true and was verified independently: internal/dataentry/feed_provider.go:296 constructs calfeed.Feed with keyed fields, never setting Component, so it defaults to ComponentEvent and every existing test passes unmodified. No regression today.

    Fix: consolidate the decision into one place on Feed (e.g. an isTodo() predicate plus a single entry-tag accessor) so the branch exists once. Separately, consider whether the mismatch state should be an error rather than silence — it is unrepresentable by intent, and silent data loss is the worst available failure mode.

    Related nit: Timed is dead state when Due is zero — Todo{Timed: true} and Todo{Timed: false} with no Due render identically and share an ETag, yet the JSON still reports allDay: !Timed, so the two representations disagree about a to-do with no date. Minor, but the kind of thing a diffing sync layer trips over.
severity: minor
resolution: |-
    Partially fixed in commit 5a0cac4e.

    DONE — the branch is consolidated. Added Feed.isTodo(); RenderCollection, CollectionTag and RenderJSON all call it rather than comparing Component themselves, so the three cannot drift and the CalDAV adapter (TKT-MF1CWZ) inherits the answer instead of reinventing it. Its doc names the three consumers.

    NOT DONE — making the Component/slice mismatch an error. RenderCollection returns []byte with no error, and adding one would change the signature for every existing VEVENT caller to guard a state no caller can currently reach (the sole external constructor, feed_provider.go:296, never sets Component). Deferring rather than reshaping a stable API for a hypothetical. The right place to catch it is config-load validation in TKT-UGYSC8, which already has to check that a vtodo collection has a vtodo-shaped mapping — noted there.

    NOT DONE — the Timed-is-dead-state-without-Due nit. Real but cosmetic; the JSON allDay flag is still meaningful as "how a due date WOULD be rendered", and changing it risks confusing a consumer that reads allDay unconditionally.
status: addressed
---
