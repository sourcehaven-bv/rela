---
id: RR-3NB0P9
type: review-response
title: unenforced{samples} risks leaking entity data (enumeration oracle)
finding: Sampling blocking values into unenforced{samples} for db status surfaces ENTITY CONTENT (secret per CLAUDE.md); checkUniqueProperties deliberately avoids this enumeration-oracle leak. State the operator-shell-only trust boundary explicitly, keep samples off any API/health surface, default to a COUNT of blocking groups, values only behind an explicit --show-values operator flag.
severity: significant
status: open
---

On CREATE-INDEX failure (pre-existing dupes), the plan samples "the blocking
values" into unenforced{reason,samples} surfaced via `rela db status`. The
blocking values are ENTITY CONTENT — the thing CLAUDE.md treats as secret ("the
configuration is not a secret; the data is"). `checkUniqueProperties`
deliberately does NOT leak colliding values/IDs to the writer (logs server-side
only, citing enumeration-oracle risk); sampling those same values into a
CLI/status surface reintroduces exactly that leak.

REQUIRED: if `rela db status`/`reconcile` is operator-shell-only (like db
migrate, no ACL, trust boundary = operator shell) the samples are acceptable
there — but the plan must STATE that boundary explicitly and ensure samples
NEVER reach an API/health surface. Bound sample count + value length;
redact/hash if any chance the surface is non-operator. Prefer reporting a COUNT
of blocking groups by default, values only under an explicit --show-values
operator flag.
