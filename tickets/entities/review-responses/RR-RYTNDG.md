---
id: RR-RYTNDG
type: review-response
title: 'queryService godoc: overstated ''holds NO store.Store'' and a doubled executeQuery lead paragraph'
finding: (1) The type doc said the leaf 'holds NO store.Store', which is true of its FIELDS but reads as absolute — executeQuery still reaches the store via svc.Store.GetEntity through the per-call Services bundle. A future reader taking the sentence literally would be confused by that line. (2) The moved executeQuery godoc carried two '// executeQuery ...' lead paragraphs, an artifact inherited verbatim from helpers.go that predates this PR.
severity: minor
resolution: '(1) Reworded to ''holds NO store.Store FIELD — it never carries an independent store handle, so a read cannot bypass the request-scoped bundle'', then states that the reads it does perform arrive through the per-call Services bundle. (2) Merged the doubled lead into one paragraph. Gates re-run on the correct branch (identity echoed alongside): build, dataentry tests, comment-lint, golangci-lint 0 issues, plimsoll.'
status: addressed
---

Minor + nit findings from the TKT-SJ0LRS code review (cranky-code-reviewer, PR
#1470).

Reviewer's deferred leverage note, NOT actioned here: the `func() T`
late-binding pattern now recurs across viewsHandler, commandHandler,
attachmentHandler and queryService, and exists solely because tests mutate App
fields post-construction — production structure shaped by test wiring. The
durable fix is a test helper that builds the App with its final collaborators,
letting every leaf hold values. Worth considering as the decomposition arc
lands; explicitly not a blocker.

Process note confirmed and worth repeating: `cd` resets between Bash calls, so a
measurement can silently land in the wrong checkout. Echo `git rev-parse
--abbrev-ref HEAD` in the SAME command as any count or gate run. This bit the
reviewer once (measuring develop's 104 instead of the branch's 86) and is the
likely cause of their incorrect 'plimsoll is inert' claim (see [[RR-LJHL3F]]).
