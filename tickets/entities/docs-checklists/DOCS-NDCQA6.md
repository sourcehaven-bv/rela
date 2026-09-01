---
id: DOCS-NDCQA6
type: docs-checklist
title: 'Docs: Require BaseDir on git.Clone'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported functions/types have godoc
- [x] Non-obvious decisions explained in comments
- [x] Package docs updated if package purpose changed

The godoc IS the deliverable here, because the defect was partly a documentation
defect: `CloneOptions.BaseDir` said containment lives at the `Clone` boundary
"so a future caller that forgets to sanitize is still safe", while the code
skipped the check when the field was empty. The comment described a guarantee
the code did not provide.

Both are now true and say so:

- **`CloneOptions.BaseDir`** — changed from "when non-empty, is the directory
Path must stay inside" to REQUIRED, and states why: the guarantee exists for the
caller who forgets, and an optional field withheld it from exactly that caller.
It keeps the concrete reason the boundary matters — `storeCredentials` writes a
plaintext OAuth token into `<Path>/.git/credentials`, so a traversal is a
credential disclosure rather than a misplaced directory.
- **`containedPath`** — records that an empty base is an ERROR, not "skip the
check", and why there is no safe default: a CWD fallback would contain the clone
somewhere the caller never named, which is a different surprise rather than a
smaller one. It also documents the root-base refusal added in review
(RR-Q5N3CJ), since "a base that contains everything" reads as an odd thing to
reject until you see it is the same silent no-op as an empty base. The existing
note about the string-level (non-symlink) threat model is preserved, since that
scope decision is unchanged.
- **`GetDefaultCloneDir`** (desktop) — documents why it returns `""` rather than
falling back to a relative path when `os.UserHomeDir` fails. A relative base
still PASSES containment, so the guard looks satisfied while the clone lands
somewhere the user never chose; the comment names that trap because the code
reads as over-cautious without it (RR-L3CC5O).
- **The `lastCloneDir` assignment** (desktop) — carries a comment on why it must
follow the successful `Clone`, since moving it back up is the natural "tidy the
locking together" refactor and would silently restore the defect (RR-HLWRMK).

The reasoning is deliberately in BOTH places. They are the two spots a person
edits when the instinct to make a required field "friendlier" strikes, and a
comment only at the call site would not be read by someone editing the field.

A test comment also carries load here. `TestClone_PathExists` now says why it
passes a `BaseDir` at all — without that note, the next person tidying the test
would drop the "unnecessary" field and silently re-break the coverage of the
`os.Stat` guard, which is exactly how this defect arose (RR-BPGG9C).

## Project Documentation

- [x] ~~CLAUDE.md updated with new patterns~~ (N/A: no new pattern — it makes
an existing containment boundary actually hold, and the `storage.RootedFS`
threat model it follows is already documented)
- [x] ~~docs/ updated for changed behaviour~~ (N/A: see Rationale)
- [x] ~~Architecture docs updated~~ (N/A: no package boundary, dependency or
wiring change; no signature change either — `BaseDir` was already a field)

## External Documentation

- [x] ~~README updated~~ (N/A: no project-level change)
- [x] ~~CLI reference updated~~ (N/A: no command or flag changes. The desktop
app's clone flow already passes a base directory and behaves identically)
- [x] ~~API docs updated~~ (N/A: `internal/git` is not a public API)

## Rationale for N/A

Nothing an operator or user can observe changes. `internal/git` is internal by
construction, `Clone` has exactly one in-tree caller
(`cmd/rela-desktop/main.go:364`), and that caller already sets `BaseDir` — so
every existing code path behaves exactly as before, byte for byte.

What changes is the failure mode for a caller that does not yet exist: instead
of silently getting no containment, it gets a clear error naming the missing
field. That audience is a future developer reading the godoc, not a user reading
`docs/`, which is why all the documentation effort went into the comments rather
than a guide.

Deliberately NOT documented anywhere user-facing: the credential-disclosure
mechanism. It is described in the godoc where a maintainer needs it to
understand why the check cannot be relaxed, but publishing "here is where the
plaintext token lands and here is the traversal that reaches it" in a user guide
would be handing over an exploitation path for no operator benefit.
