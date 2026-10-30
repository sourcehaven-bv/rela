---
id: RR-EMV5XI
type: review-response
title: Scope is l, not m; split into three PRs
finding: 'The plan touches realtimecfg (a move, a new checker, raw and key changes), realtime (views, affordances, the session read), apiwire, the dataentry wire and goldens, openapi, the MCP descriptions and golden, four SPA files plus ChannelsPanel tests, and two docs. Its test list holds 12 groups. Split it: (1) the value-rule move as a pure refactor with the arch-lint change, which is easy to review because no behavior changes; (2) the server side: the button key, the load checks, the served button, _actions.entries, OpenAPI, MCP and the realtime.md key and API sections; (3) the SPA presets and the ''In the web app'' docs. PR 2 can merge first, because older SPAs ignore the new fields.'
severity: significant
reason: Effort is raised to l, and the work stays one ticket and one PR, with a commit per layer so the value-rule move can be reviewed on its own. The three-PR split is declined.
status: wont-fix
---
