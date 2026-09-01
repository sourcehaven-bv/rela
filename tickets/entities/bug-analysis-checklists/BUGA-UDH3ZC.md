---
id: BUGA-UDH3ZC
type: bug-analysis-checklist
title: 'Analysis: yaml.v3 emits a block scalar it cannot re-parse for leading-newline strings'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

Minimal, no rela involved:

```go
raw, _ := yaml.Marshal([]string{"\n0"})
// raw == "- |4-\n  0\n"   <- indent indicator says 4, body is indented 2
yaml.Unmarshal(raw, &back)
// yaml: line 1: did not find expected '-' indicator
```

Through rela: ~50s of `go test -fuzz` on `FuzzPropertyValuesTypeZoo`. The
crashing input is `string("0") / int(52) / string("\n0")` — an ordinary entity
property value.

Characterized before fixing. Only a leading newline FOLLOWED BY CONTENT trips
the emitter:

| value | emitted | reads back |
| --- | --- | --- |
| `"\n0"` | `- \|4-\n  0\n` | **error** |
| `"\nx"` | `- \|4-\n  x\n` | **error** |
| `"\n"` | `- \|4+\n` | **`""` — silent data loss** |
| `"a\nb"` | `- \|-\n  a\n  b\n` | ok |
| `"0\n"` | `- \|\n  0\n` | ok |
| `" x"` | `- ' x'\n` | ok |

The `"\n"` row was NOT in the original report and is worse than what was
reported: it never errors. It emits `|4+` and reads back as `""`, losing the
value silently. Found only because the probe compared values rather than
checking for an error — which is exactly the trap the fix's test guards.

**Scope, and both halves matter for the fix decision:**

- **fsstore only.** `pgstore.marshalProps` (`internal/store/pgstore/entity.go:630`)
uses `json.Marshal`; JSON has no block scalars and the value round-trips.
- **Frontmatter only.** `formatEntity` (`markdown.go:292`) writes `e.Content`
raw beneath the fence; the body never passes through YAML. Multi-line prose
belongs there and is untouched.

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

**why1** — `yaml.Unmarshal` rejects `yaml.Marshal`'s own output for these
strings.

**why2** — The emitter writes a block-scalar header whose explicit indent
indicator (`4`) disagrees with the indentation it then writes (`2`).

**why3** — It only miscounts when the first character is a newline. With content
on the first line the indent is derived correctly; a leading newline makes the
emitter compute the indicator from a line that has no content.

**why4** — This is an upstream `gopkg.in/yaml.v3` defect, not a rela bug. rela's
role is that it hands arbitrary user strings to a library whose round-trip
property it assumed rather than verified. The assumption held for every value
anyone had tried.

**why5** — The systemic issue is that a serialization boundary was treated as
total. `valueToNode` had no notion that some values are unrepresentable in the
target format, so there was no place for the question "what if this does not
round-trip?" to be asked. The fuzz sweep exists precisely to find where that
assumption breaks, and it did.

Worth separating: the failing WRITE was the safe outcome throughout. A fix that
merely silenced the error while still emitting unreadable YAML would have turned
a loud failure into a corrupt file. That shaped the whole approach.

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

**Approach: quote the scalar (option D), not reject it (option A).**

The initial inclination was A — reject with a clear error — reasoning that a
leading newline in a frontmatter scalar is not content anyone authors
deliberately. **That was wrong**, and the pgstore fact is why: rejecting would
make the two backends disagree about what a valid entity IS. The same `PATCH`
would succeed on Postgres and 400 on fsstore. A storage-layer serialization
limit would become a data-validity rule enforced by only some deployments.

It also breaks the invariant a store should hold: persist what you are given or
fail loudly; do not develop an opinion about which strings are worth keeping.

Built the quoted node BEFORE `Encode` rather than fixing up its result —
`Node.Encode` round-trips internally and returns the error itself, so there is
no node to post-process. Discovered by trying the post-process version first and
watching it fail identically.

**Regression tests.** `TestValueToNode_RoundTrips` asserts marshal → unmarshal →
COMPARE across all six characterized shapes plus list variants. Comparing values
rather than checking for an error is load-bearing: it is what catches the `"\n"`
data-loss case, which a write-only assertion would call a pass.
`TestValueToNode_LeavesOrdinaryMultilineAlone` pins that ordinary multi-line
strings keep block style, so the fix does not reflow files it need not touch.
The original crashing seed is now committed under `testdata/fuzz/` and passes.

**Related areas — two found:**

1. `valueToNode` was DUPLICATED in `internal/markdown/parser.go` and
`internal/store/fsstore/markdown.go`. The report warned about this and it was
real: fixing one and re-running the fuzz target still failed. Rather than copy
the fix, fsstore now delegates to an exported `markdown.ValueToNode`, so entity
writes and relation writes cannot diverge again. arch-lint already permits that
dependency and fsstore already imported the package.

2. Re-fuzzing for 90s after the fix surfaced a DIFFERENT defect, filed as
BUG-X7ICNM: for invalid UTF-8, fsstore correctly refuses (`cannot marshal
invalid UTF-8 data as !!str`) while pgstore silently substitutes U+FFFD and
reports success. That is this bug's mirror image — there the value is genuinely
unrepresentable and fsstore is right, so the fix direction reverses: make
pgstore stricter, not fsstore laxer. Seed parked at
`.ignored/issue-round/fuzz-utf8/`, not committed, since alone it is a
deliberately-failing test.
