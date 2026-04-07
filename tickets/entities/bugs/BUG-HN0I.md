---
id: BUG-HN0I
type: bug
title: CLI hides real project-discovery errors behind generic 'no project found' message
description: 'The PersistentPreRunE hook in internal/cli/root.go wrapped every error from workspace.Discover as ''no project found: run rela init to create one'', hiding real failures like metamodel parse errors, permission errors, or pending migrations. Fixed by distinguishing errors.ErrNoProject (shows init hint) from other errors (surfaced verbatim).'
priority: medium
why1: Every discovery error was mapped to a single hardcoded user-facing string, discarding the underlying cause.
why2: The hook did not distinguish between genuinely missing projects and other failures.
why3: No sentinel check was used; the code chose a friendly hint over correctness without fallback for other cases.
why4: Error messages were treated as a nice-to-have polish item rather than a debugging surface, so helpful hints were added without preserving the underlying cause.
why5: No coding guideline in the project requires that user-facing error translations fall through to the wrapped error for unexpected failure modes.
prevention: When translating internal errors to user-facing messages, always check a sentinel with errors.Is before swapping in a generic hint, and fall through to the wrapped error for any other case.
status: done
---
