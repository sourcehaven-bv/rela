---
id: RR-4QQO65
type: review-response
title: The plan claims a poll on the page that does not exist
finding: The out-of-scope note says a post from another process is caught by "the next full round or the next poll on the page". The plan adds no poll, and docs/realtime.md:1739-1762 says rela-server never polls for writes from other processes. Agents on stdio rela mcp are a main producer of mentions and waiting sessions, so in a quiet project the badge can stay stale for a long time. Either add a slow safety replay of inbox subscriptions on the fixed grid (the MCP watch uses inboxPollEvery = 15 s, tools_inbox.go:25), or correct the text and document the staleness in docs/realtime.md and data-entry.md.
severity: minor
resolution: Added a safety replay of all inbox subscriptions with a wake set every 30th tick (15 s) on the grid, removed the claim of a page poll, and documented the up-to-15 s delay for writes from other processes (AC 16).
status: addressed
---
