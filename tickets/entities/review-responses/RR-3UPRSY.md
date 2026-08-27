---
id: RR-3UPRSY
type: review-response
title: Block spinners blanked entity detail, edit form and search on every load
finding: |-
    Found by the user driving the running demo at injected latency.

    The migration had gated and restyled BUTTONS but left every block spinner ungated. `EntityDetail.loadView()` set `loading = true` unconditionally, and the template's `v-if="loading"` replaced the entire article with a centred `.loading-state`. So stepping prev/next collapsed the page from ~2300px to ~140px and sprang it back on every step — the single worst layout shift in the app, and on a fast connection a flicker rather than a loading state.

    Same shape in DynamicForm (opening an edit form) and SearchView (re-running a search). SearchView was additionally showing TWO indicators for one act: the Search button's pending label and a block spinner replacing the results, violating the ticket's own one-indicator-per-user-act rule.
severity: significant
resolution: |-
    Fixed per surface, because the right answer differs by surface rather than being one uniform gate:

    - **EntityDetail** — keeps the current entity rendered while the neighbour loads (`loading && !viewData`), so the block spinner is reachable only on a genuine cold load, and gated on top of that.
    - **DynamicForm** — no previous content exists to hold (a form is either opening or not), so the delay gate alone is the fix.
    - **SearchView** — block spinner REMOVED entirely. The button already owns the operation, and previous results now stay on screen while the next query runs.
    - **DocumentView / DocumentsPanel** — keep their pre-existing `!docContent` guard (RR-TCZWUI) and gain the gate.

    Verified in Chromium at 1500ms injected latency by sampling the DOM across a full next-step: the article is present in EVERY frame, zero `.loading-state` elements appear, and body height moves ~100px instead of collapsing.

    Two self-inflicted errors during this work, both caught by the suites rather than shipped. (1) A first attempt deleted the spinner elements with a regex, which orphaned the following `v-else-if` branches and broke 62 unit tests — reverted and redone per-site preserving each branch chain. (2) A bulk edit inserted the gate ABOVE the refs it reads, producing a temporal-dead-zone ReferenceError in DocumentsPanel that failed 63 e2e tests; fixed by moving each gate below its dependencies and then checking declaration order across all four gated files.
status: addressed
---
