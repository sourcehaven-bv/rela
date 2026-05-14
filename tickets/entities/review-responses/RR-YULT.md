---
id: RR-YULT
type: review-response
title: MAX_RESULTS=50 client cap silently truncates with no UX indication
finding: |-
    `EntityPickerModal.vue` caps results with `resp.data.slice(0, MAX_RESULTS)`. The backend `handleV1Search` returns ALL matches (no `per_page` honored — see `internal/dataentry/api_v1.go:1140`). On a project with many entities matching a common query (`fix`, `add`, `test`), the user sees exactly 50 results and has no signal that more were truncated. They will type more refining characters thinking the list is small, but they may be hiding a relevant result that was at rank 51.

    The ticket-summary justification 'acceptable for a picker' is defensible — 50 is plenty when ranked by relevance — but the silent truncation is the actual UX bug. CommandPaletteModal has the same shortcoming (it's where this code was copied from), so this is also a pre-existing issue surfacing in the new component.

    Minimum fix: show a hint row when `resp.data.length > MAX_RESULTS` saying 'Showing first 50 of N matches — refine your query'. The data is already in `resp.data` so no extra round-trip is needed.

    Better fix: thread `per_page` through `/_search` server-side so the backend doesn't hand the SPA a payload it's going to discard. The handler already builds a `V1ListMeta` with `Total`/`PerPage` — wire it to a query param like the list endpoints already do.
severity: minor
reason: Inherited from CommandPaletteModal. Adding a 'showing N of M' hint requires the backend to return total counts on /_search (today it doesn't). Out of scope for the picker; would need a backend change. A future ticket can add the hint in lockstep across both palettes.
status: deferred
---
