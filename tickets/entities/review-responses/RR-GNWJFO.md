---
id: RR-GNWJFO
type: review-response
title: Comment falsely claims TypeScript enforces the no-description-alias rule
finding: 'config.ts JSDoc, the KanbanConfig comment, and config.test.ts all claim the config TYPE prevents the description fallback reaching kanban. TypeScript types erase at runtime, so viewHeaderMarkdown({description: ''LEAKED'', ...}) returns ''LEAKED''. Configs arrive from the /_config HTTP response (runtime data), so if Description is later added to the Go Kanban struct, kanban silently gains the deliberately-excluded fallback while the comment says that is impossible.'
severity: significant
resolution: 'Turned the policy into a mechanism instead of a comment. viewHeaderMarkdown now takes an explicit opts.allowDescriptionAlias flag; only EntityList passes it, kanban never does, so the alias no longer depends on a type that erases at runtime. Rewrote all three false comments (config.ts JSDoc, KanbanConfig, config.test.ts) to state that the call site enforces it. Added two regression tests: the alias is ignored when not opted into, and a kanban-shaped object carrying a stray `description` (the exact drift scenario — a future Go Kanban.Description field) resolves to ''''. Verified end-to-end: list `active_ideas`, which has only `description` and no `header`, still renders its region against the real server, while all six kanbans expose no description key.'
status: addressed
---

## Finding

`frontend/src/types/config.ts:169-172` (JSDoc), `:255-256` (KanbanConfig
comment) and `config.test.ts:121-122` all assert that the config *type* is what
keeps the `description` fallback list-only. TypeScript types are erased at
runtime, so this is false:

```ts
const board = { entity: 'ticket', column_property: 'status', description: 'LEAKED' }
viewHeaderMarkdown(board) // => 'LEAKED'
```

The structural param accepts any object carrying a `description`. Board configs
arrive from the `/_config` HTTP response — runtime data. If someone later adds
`Description` to the Go `Kanban` struct (the field name sits right there on
`List`, one struct over), kanban silently gains the fallback the ticket
deliberately excluded, while the comment says that cannot happen.

The design decision is sound; the enforcement story is wrong. A false-confidence
comment is worse than no comment.
