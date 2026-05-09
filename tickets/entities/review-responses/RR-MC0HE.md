---
id: RR-MC0HE
type: review-response
title: idKeyRe is far looser than the docs imply
finding: '^[A-Za-z0-9](?:.*[A-Za-z0-9])?$ accepts any character except \n in the middle: tabs, |, (, ), spaces, $, etc. Docs imply alphanumeric/hyphen IDs. Mostly safe due to regexp.QuoteMeta but surprising: a key with \t never matches real text, a key with a space could accidentally match across word boundaries. Tighten to match the boundary-char class.'
severity: significant
resolution: Tightened idKeyRe to ^[A-Za-z0-9]([A-Za-z0-9_-]*[A-Za-z0-9])?$. Added negative tests for keys containing tab, space, and pipe. Updated docs to state the constraint precisely.
status: addressed
---

# Finding

`idKeyRe` (`internal/lua/markdown.go:1344`):

```go
var idKeyRe = regexp.MustCompile(`^[A-Za-z0-9](?:.*[A-Za-z0-9])?$`)
```

The middle `.*` allows any character except `\n`: `\r`, `\t`, `\0`, `|`, `(`,
`)`, `$`, spaces, etc. Docs imply alphanumeric/hyphen-only.

Mostly safe because keys are regex-escaped before alternation, but surprising:

- Key with `\t`: never matches real text (silent no-op).
- Key with space: matches across word boundaries (because boundary char
class includes space).
- Key with `|`: escaped, fine in pattern, but obscure to debug.

# Resolution

Tighten to match the boundary class:

```go
var idKeyRe = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9_-]*[A-Za-z0-9])?$`)
```

(Also covers single-char keys via the optional group.)

Update docs to state the constraint clearly.

Add negative tests for keys containing `\r`, `\t`, `|`, space.
