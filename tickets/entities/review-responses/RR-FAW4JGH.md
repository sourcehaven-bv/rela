---
id: RR-FAW4JGH
type: review-response
title: Rate limit, mention fan-out and late answer edits
finding: Tokens are taken per message before validation; mentions cap per message multiplies by batch size; earlier answers stay editable after the move.
severity: minor
resolution: Tokens are charged in one step after the whole batch validates (limiter.takeAll); mentions are capped per batch. Answer edits after the move stay allowed and are documented.
status: addressed
---
