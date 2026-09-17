---
id: IMPL-708FNA
type: implementation-checklist
title: 'Implementation: Replace EasyMDE with Milkdown (ProseMirror) in data-entry forms'
status: done
---

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Edge cases from planning, each with the mechanism that handles it:

| Planned edge case | Handling |
| --- | --- |
| Empty document | real `.milkdown-placeholder` element, no inline `style` (app CSP) |
| Table normalization at load | `armed` flag, set once loading settles |
| Just-inserted ref absent from the mentions map | `keepExisting` when the node has no server provenance |
| ID shape passes but entity does not exist | renders the bare ID |
| `@` inside an email address | boundary character required before the trigger |

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

The entity-ID grammar uses a shared JSON fixture read by both the vitest suite
and a Go test, rather than two hand-maintained lists.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Ran against a real server (`rela-server` on a scratch project, ACL disabled for
the session) and drove it in Chrome. Found and fixed four defects that no test
had caught, each now with a regression test:

1. `@` menu rendered at the bottom of the editor instead of at the cursor.
`SlashProvider` positions `options.content`, which was a static wrapper `<div>`
inside the contenteditable while the coordinates were styled on the panel inside
it.
2. An inserted reference displayed as a bare ID. `resolveAttrs` blanked the
title the picker had just supplied, because a fresh ID cannot be in the
load-time mentions map.
3. Every toolbar command was dead. The catalogue read `.key` off the imported
command objects, but `$command` assigns `plugin.key` inside the function it
returns, so at module load it is `undefined` and `callCommand(undefined)` threw
inside the ctx container.
4. Toolbar showed stale active state until the first edit, because
`appendTransaction` never runs for the document the editor loads with.

Acceptance criteria:

1. **Open and save emits nothing** — full corpus sweep green (3,939 files, 0
semantic drift), plus component tests over four churning body shapes asserting
no emit. Note this criterion was NOT actually met until the design review; see
RR-6OIIOE and RR-P3K8UG.
2. **Reference renders as a title, serializes as a code span** — verified in
the browser (displayed "Milkdown editor demo") and on disk (the file contained
exactly `` `TKT-007` ``).
3. **No title leaks through the read gate** — Go tests, proven non-vacuous by
substituting `visibility.AllowAllReader `: both LEAK assertions fire with the
hidden title in the response body.
4. **App-editor bundle unchanged** — still builds on EasyMDE; SPA bundle
contains no `EasyMDE `.

Not verified in a browser: the toolbar's final appearance after the last round
of styling changes, because the Chrome extension stopped connecting. Behaviour
is covered by tests and the built assets were checked over HTTP.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Patterns followed rather than reinvented: `.md-body ` from the shared markdown
stylesheet supplies all typography; design tokens from `scales.css `
throughout; `aria-disabled ` rather than native `disabled `, per the focus
rule in `frontend/CLAUDE.md `; `collectMentions ` reused rather than a second
scanner.

Command availability is a dry run against the commands themselves rather than
hand-written predicates, so the disabled state cannot drift from behaviour. The
exceptions are documented: three guards in `tableCommands.ts ` exist because
the ProseMirror table commands report success and then corrupt a GFM table.

Silent failures fixed during review: `drift-blocked ` dropped the user's edit
with no feedback (RR-X4K509). The E2E hook is compiled out of production builds
by `__E2E_TEST_HOOKS__ `; verified 0 occurrences in the production bundle.
