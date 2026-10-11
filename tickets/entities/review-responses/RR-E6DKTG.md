---
id: RR-E6DKTG
type: review-response
title: Visible sources behind a long hidden run could not be reached
finding: After the cursor fix, a budget stop resumed after the last served id, so the next call rescanned the same hidden rows and never reached a visible source beyond them.
severity: significant
resolution: 'A budget stop no longer hands back a cursor; it sets Truncated. The budget is raised to 10,000 rows (batches of 500), so only a target mentioned by more than 10,000 unreadable entities in a row is affected, and every surface says so (REST/Lua truncated, CLI/MCP mentioned_by_truncated, SPA note). The lookahead item makes Next mean more served sources exist. Test: TestResolver_MentionedByPagesPastHiddenRun.'
status: addressed
---
