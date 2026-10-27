---
id: RR-E2SF4K
type: review-response
title: Bare-string rule misses literals in bound expressions
finding: 'vue/no-bare-strings-in-template checks text and static attributes only. A literal inside a binding such as :title="a ? ''Syncing...'' : ''Click to sync''" passes the guard.'
severity: significant
resolution: 'Plan: convert those by hand in the slice and state the limit in frontend/CLAUDE.md so reviewers watch for it.'
status: addressed
---
