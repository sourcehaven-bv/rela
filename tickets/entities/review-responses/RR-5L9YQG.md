---
id: RR-5L9YQG
type: review-response
title: Scope is size l, not m; split into two PRs
finding: 'Files to modify (plan lines 505-534) span ~40 files over realtimelimits, realtimecfg (8 files), realtime, apiwire, dataentry, openapi + goldens, mcp, cli, the app SPA and the component library, with 13 ACs and two new frontend APIs (DecisionComposer.reply/prefill). The list property alone (config, validation, predicate type, JSON Schema, wire, OpenAPI, CLI append/render, SPA render/edit, every typeof-object site) is a full m. Split: PR 1 = `list: true` string properties end to end (AC 1-3, 7, 12, with composer entry deferred, see the composer finding); PR 2 = `choices:` config, descriptors and buttons (AC 4-6, 8-11, 13). The plan''s own fallback (line 719-722) already admits the composer part may not fit.'
severity: significant
reason: The owner keeps this as one ticket and one PR. Instead the plan cuts composer entry of lists, DecisionComposer.prefill, threaded questions and markdown into, trims padded prose, and raises the effort to l.
status: wont-fix
---
