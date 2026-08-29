---
id: RR-1YIOG5
type: review-response
title: 'Shipped docs example uses icon: progress, which the allowlist rejects — the generated table made the hand-written examples beside it more trusted, not less'
finding: |-
    `docs/data-entry.md:2004` and its source `GUIDE-data-entry.md:2010` document a kanban example containing `icon: progress`. `progress` was not in the old 16-name allowlist and was not in the new generated one either — an author copy-pasting the guide's FIRST icon example cannot start the server.

    Pre-existing rot, but this ticket rewrote the surrounding prose and regenerated the table directly beside it without catching it. Worse, the generated table makes the whole page read as machine-checked, so a reader now trusts the hand-written example MORE than before.

    `internal/dataentry/nav_icon_test.go:46-47` compounded it by asserting `Icon == "progress"` round-trips. It passed because `navEntryToSidebarItem` does no validation (correct layering), which means the test certified an unreachable name as working.

    RR-GTOQCF was "the docs name list went stale". Generating the list while leaving the EXAMPLES hand-written moved that bug twenty lines up the page rather than eliminating it.
severity: critical
resolution: |-
    Added `progress` (LoaderCircle, "Work underway; a partial ring") — there was a real gap between `active` (started) and `done`, which is why the example reached for it.

    More importantly, added `TestDocsExamplesUseValidNames` in cmd/gen-icons: it extracts every `icon: <name>` from both the guide source and the rendered docs and asserts each is in the allowlist (or is the reserved `none`). Mutation-verified — removing `progress` from the table makes it fail on both files with the copy-paste consequence spelled out. This closes the class permanently rather than fixing the one instance.
status: addressed
---
