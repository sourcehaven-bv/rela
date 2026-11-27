---
id: RR-3FSJWN3
type: review-response
title: Skip rule is broader than its comment
finding: Commits with SQL, testdata or .golangci.yml changes but no .go file skip lint and tests, while the comment said only ticket or doc files skip.
severity: minor
resolution: The comment and skip message now name the trigger files exactly; .golangci.yml now triggers the linter.
status: addressed
---
