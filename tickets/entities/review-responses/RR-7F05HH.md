---
id: RR-7F05HH
type: review-response
title: AC7 'sanitised the same way as User/Tool' is not implementable for []string; RawUser already unsanitised
finding: 'AC7 says roles are sanitised the same way User/Tool are (audit/filesystem.go:204). clean() takes and returns a string (filesystem.go:227) — it cannot sanitize []string, so AC7 as phrased is satisfiable by doing nothing. A per-element loop is required. On the injection question the plan''s implicit worry is overstated: the JSONL writer marshals via encoding/json which escapes newlines inside strings, so a crafted role cannot forge a log line. But clean()''s real jobs still apply per element: the 1024-rune fieldLimit cap (filesystem.go:183) and control-char replacement. Separately, RawUser is NOT sanitised at filesystem.go:204-205 — currently safe by construction because it is only ever set from an already-sanitizeUser''d value at router.go:280, but that is an unpinned invariant sitting in the exact function this ticket edits.'
severity: significant
resolution: 'Confirmed: clean() is func(string) string at filesystem.go:227 and cannot take []string, so AC7''s ''sanitised the same way as User/Tool'' was satisfiable by doing nothing. AC7 restated concretely: sanitize() gains a per-element loop applying clean() to each role plus the element-count cap from RR-0VHKMW, and the org fields get clean() like User/Tool. The reviewer''s correction is accepted: encoding/json escapes newlines inside strings, so a crafted role cannot forge a JSONL audit line — the log-injection worry is overstated and is NOT the justification; the real reasons are the 1024-rune fieldLimit cap (filesystem.go:183) and control-char replacement. RawUser is also added to sanitize() while in the function: currently safe by construction (only ever set from an already-sanitizeUser''d value at router.go:280) but an unpinned invariant in the exact function this ticket edits, and one line to close.'
status: addressed
---

## Finding

`sanitize` (`audit/filesystem.go:199-209`):

```go
rec.Principal.User = clean(rec.Principal.User)
rec.Principal.Tool = clean(rec.Principal.Tool)
```

`clean` takes and returns a `string` (`filesystem.go:227`). **It cannot sanitize
`[]string`.** AC7's "sanitised the same way `User`/`Tool` are" is therefore not
an implementation — as phrased it would be satisfied by doing nothing.

**On log injection: the plan's implicit worry is overstated.** The JSONL writer
marshals via `encoding/json`, which escapes `\n` inside strings, so a newline in
a role does *not* forge an audit line. But `clean`'s actual jobs still apply per
element: the 1024-rune cap (`fieldLimit`, `filesystem.go:183`) and control-char
replacement.

**Adjacent pre-existing gap:** `RawUser` is not sanitised at
`filesystem.go:204-205`. It is currently safe by construction — only ever set
from an already-`sanitizeUser`'d value at `router.go:280` — but that is an
unpinned invariant sitting in the exact function this ticket edits.

## Resolution

- Restate AC7 concretely: a per-element loop applying `clean` to each role,
plus the element-count cap from the roles-bound finding.
- Fix `RawUser` while in the function; one line, and it removes a latent trap.
