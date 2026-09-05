---
id: RR-K7RMIC
type: review-response
title: Counting matched entities makes require_visible_content lie for detail sections whose entities render empty
finding: 'The plan defines emptiness as count == 0 where count is MATCHED entities, and documents that a detail section whose entity has empty Content yields count 1 and is therefore SENT. That produces exactly the ''Nothing to show.'' message the ticket exists to prevent, so the feature fails its stated purpose in a case the plan itself identified and then rationalised. style: detail is the style in the ticket''s own motivating Atlas example (mt_overleg), so this is not a corner case. Fix: count CONTRIBUTIONS rather than matches, inside the existing Build loop - table/default and list always contribute; detail contributes only when strings.TrimSpace(ent.Content) != "". This must be a SECOND counter, not a redefinition: {{count}} legitimately means ''entities matched'' and must keep its current semantics.'
severity: significant
resolution: 'Plan updated before implementation. Build now returns a CONTRIBUTION count rather than a match count: table/default and list always contribute, detail contributes only when strings.TrimSpace(ent.Content) != "". Implemented as a second counter so the existing count keeps its ''entities matched'' meaning for {{count}} interpolation, which must not change. Edge-case table updated to assert the detail-with-empty-Content case is now SUPPRESSED when opted in, plus an assertion that {{count}} still expands to 1 in the opted-out case, pinning the two counters as distinct.'
status: addressed
---

## Finding

The plan defines emptiness as `count == 0`, where `count` is matched entities
across all sections, and explicitly documents this edge case:

> `detail` section whose entity has empty `Content` → count 1, so SENT. The
> entity matched; the operator's template chose to render nothing.

That reasoning is wrong on its own terms. The setting is named
`require_visible_content` and the ticket's stated problem is a recipient
receiving a message that renders as "Nothing to show." A `detail` section whose
matched entities all have empty `Content` produces **exactly that message** —
the section body is empty — yet the plan sends it.

So the feature fails to fix the reported problem in a case the plan already
identified, and then rationalises it. `style: detail` is the style used in the
ticket's own motivating Atlas example (`mt_overleg`, `style: detail`), so this
is not a corner.

## Why "count matched entities" was chosen, and why that part is still right

The plan rejects inspecting the rendered `mailrender.Section` because emptiness
lives in different fields per style (`Body` for `detail`/`list`, `Rows` for
`table`). That objection is sound — a predicate over the rendered message would
need updating for every new style.

The flaw is not *where* the signal comes from, it is *what is counted*. The
builder can count contributions rather than matches, at the same place in the
same loop, without inspecting the rendered output afterwards.

## Resolution

In `mailtemplate.Build`, count an entity only when it actually contributes
content to its section:

- `table` (and default): always contributes — a row exists.
- `list`: always contributes — a link line is written.
- `detail`: contributes only when `strings.TrimSpace(ent.Content) != ""`.

Keep the existing `count` semantics for `{{count}}` interpolation, which
legitimately means "how many entities matched" and must not change — so this is
a **second** counter, not a redefinition of the first. `Build` then returns the
contribution count for the send decision while `{{count}}` keeps expanding to
the match count.

Add a test: `detail` section, one matched entity with empty `Content`,
`require_visible_content: true` → suppressed. And assert `{{count}}` still
expands to `1` in the opted-out case, pinning that the two counters are
genuinely distinct.
