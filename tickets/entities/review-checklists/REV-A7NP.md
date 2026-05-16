---
id: REV-A7NP
type: review-checklist
title: 'Review: Lift workspace.RenameEntityType to internal/renametype'
status: done
---

## Code Review

- [x] cranky-code-reviewer run on the diff
- [x] No critical findings
- [x] 2 significant findings addressed in-PR (wiring discipline → panic-at-call-time; template/MkdirAll error surfacing + atomicity doc)
- [x] 3 minor findings addressed (0o-prefix octals, godoc accuracy, CRLF test case, nil-deps zero-value doc)
- [x] 3 won't-fix (deps-access style intentional; YAML AST vs byte preservation intentional; Service vs free fn — consistency with attachment)
- [x] 1 deferred (fluent test builders — pattern propagation tracked separately)
- [x] Tests pass under `-race`
- [x] `just ci` green

## Disposition

See IMPL-JT32 for the full table.

**Highlights of significant wins:**

- **Wiring discipline.** First draft conditionally skipped renametype wiring when `ws.FS()` was nil (test fixture gap). Reviewer correctly flagged this as soft-failure that invites drift. Replaced with a panic at call time so misconfigured test fixtures fail loudly.
- **Atomicity transparency.** The 4-step rename (metamodel / dir / per-file / template) is not atomic. Previously, mid-flight failures returned vague errors and template-rename errors were silently swallowed. Now each error message names what already succeeded, and the godoc states "NOT ATOMIC" explicitly.
- **CRLF documentation.** Found via review: the rewritten line loses `\r` while surrounding lines keep theirs. Test case added that pins current behavior — any future change is intentional.
