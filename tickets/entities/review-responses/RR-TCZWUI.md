---
id: RR-TCZWUI
type: review-response
title: Migration list would regress DocumentView/DocumentsPanel, which already implement the correct keep-content guard
finding: |-
    The plan's migration list names DocumentView.vue:168 and DocumentsPanel.vue:189 as block-spinner sites to "gate or delete". Both are ALREADY correct and are the in-repo precedent for the behaviour the ticket wants:

        <div v-if="loading && !docContent" class="loading-state">

    The `&& !docContent` is exactly keep-previous-content: a refresh never blanks the rendered document, and the block spinner appears only on a cold load. That is the same semantic as Colada's isPending, hand-rolled. Mechanically gating these behind useDelayedPending without preserving the `!docContent` condition would REINTRODUCE blanking on refresh — a regression the ticket exists to prevent.

    Separately, both files have an inline `.spinner-sm` at :162 / :183 inside the refresh button. Those are the icon-only-control case the ticket carves out as an exception, not block spinners — the migration list should distinguish them, since "gate or delete" is the wrong instruction for both.

    Recommend: annotate these two sites in the plan as "already correct, preserve the !docContent guard", and split the icon-only spinners into their own line item.
severity: minor
resolution: Both sites re-annotated in the plan's Files-to-modify list. DocumentView.vue:168 and DocumentsPanel.vue:189 are marked 'already correct — PRESERVE the && !docContent guard', with an explicit warning that removing that condition would reintroduce blanking on refresh, the exact regression this ticket exists to prevent; adding the delay gate there is optional polish rather than required work. They are also promoted in the Research section to 'the in-repo precedent for keep-previous-content, hand-rolled — same semantic as Colada's isPending', alongside AutoSaveIndicator as a pattern to generalize from rather than replace. The inline .spinner-sm at DocumentView.vue:162 / DocumentsPanel.vue:183 is split into its own line item as the icon-only-control case (swap the icon in place in the same box), since 'gate or delete' was the wrong instruction for it.
status: addressed
---
