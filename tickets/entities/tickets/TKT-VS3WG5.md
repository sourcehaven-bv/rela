---
id: TKT-VS3WG5
type: ticket
title: Block agent session-attribution trailers from git history
kind: enhancement
priority: low
effort: s
status: backlog
---

Agent sessions append an attribution trailer (`Claude-Session: <url>`) to commit
messages. 192 of them are already in this repo's history, 102 of them reachable
from `develop`. They carry no value for a reader of the log and should not keep
accumulating.

## Why not a ruleset

The obvious mechanism does not work. GitHub's `commit_message_pattern` metadata
restriction was tested at BOTH repository and organization level on this org
(Team plan): the API accepts the rule, reports `enforcement: active` and
`current_user_can_bypass: never`, and `GET /repos/.../rules/branches/main` lists
it as an evaluated rule — and a push carrying the trailer is accepted anyway.
Reproduced with `contains` and with anchored `regex`, and with the pattern in
the subject line rather than the body. Metadata restrictions appear to be
Enterprise-gated; the API gives no error, so a rule that blocks nothing looks
correctly configured in the UI.

Server-side `pre-receive` hooks are Enterprise Server only, so they are not an
option either.

## What this does instead

A reusable workflow in `sourcehaven-bv/.github`, called by a small caller
workflow per repo, fails the PR when the trailer appears in:

- any commit on the PR branch,
- the PR title,
- the PR **body**.

The body matters and is the reason a commit-message rule would have been
insufficient even if it worked: `develop` merges via squash/merge-queue, and
squash composes the merge commit message from the PR title and body. A trailer
there reaches `develop` even when no individual commit carries one.

Matching is anchored to the trailer KEY at line start
(`^\s*Claude-Session\s*:`), so prose or a bare session URL mentioning the token
does not trip it — this ticket's own PR body would otherwise fail.

## Enforcement note

Making the check required needs the qualified status context
`no-session-trailer / No Session Trailer` — a reusable workflow reports as
`<caller-job> / <job-name>`. Requiring the bare job name yields a PR that is
green and permanently unmergeable.

Complementary local guard: a `PreToolUse` hook blocks the agent writing the
trailer in the first place, so CI is the backstop rather than the only line.
