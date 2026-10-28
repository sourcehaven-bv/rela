---
id: RR-71RYP7
type: review-response
title: Split the SPA picker into its own ticket now
finding: 'The plan has 22 ACs across four store runs, two schema rungs, parser, classifier, inbox, render, REST, OpenAPI, MCP, CLI and SPA. It says itself that it is at the upper edge of L. The picker (AC 16) is a self-contained UI unit with its own a11y and play tests, and nothing on the server depends on it. The schema `mentions` block (AC 17) only feeds the picker. Recommendation: move AC 16 and AC 17 to a follow-up ticket. Keep AC 15 (chips) here, so the `mentions` wire field has a consumer and the golden and TS mirror land together. Agents, the CLI and people typing `` `USR-7` `` by hand work without the picker.'
severity: minor
resolution: Picker (AC 16) and schema mentions block (AC 17) moved to follow-up a; chips (AC 15) kept; ACs renumbered to 20.
status: addressed
---
