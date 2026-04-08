---
id: BUG-8D16G
type: bug
title: Pre-push hook rejects follow-up work on existing tickets
description: 'The pre-push hook in .git/hooks/pre-push requires a ticket/bug/feature markdown file with --diff-filter=A (Added) in every push that touches code. This correctly gates the initial PR that files a ticket, but it rejects follow-up PRs that continue work on an existing ticket — because the ticket file lives on develop from the original PR and shows as Modified (M), not Added (A), in the follow-up push. Observed while pushing TKT-LK1J (logger DI refactor): the ticket TKT-LK1J.md was filed in PR #339 and merged to develop. The follow-up refactor PR modifies the ticket body (status transitions from backlog -> ready -> done) but the hook refuses because TKT-LK1J.md isn''t ''added'' in the current push.'
priority: low
effort: xs
why1: Hook checks git diff --diff-filter=A (Added only) when looking for a ticket entity in the push.
why2: Added-only filter assumes every code-changing PR is the one that originally files the ticket.
why3: The workflow supports ticket lifecycles that span multiple PRs (ticket filed in PR N, implemented in PR N+1) but the hook does not model this lifecycle.
status: backlog
---
