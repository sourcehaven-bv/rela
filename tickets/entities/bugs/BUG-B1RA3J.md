---
id: BUG-B1RA3J
type: bug
title: yaml.v3 emits a block scalar it cannot re-parse for leading-newline strings
description: 'gopkg.in/yaml.v3 emits a block scalar it cannot read back for a string beginning with a newline: yaml.Marshal([]string{"\n0"}) produces "- |4-\n  0\n", where the indent indicator says 4 and the body is indented 2, and Unmarshal rejects it. A lone "\n" is worse -- it emits |4+ and reads back as "", losing the value silently. fsstore only (pgstore uses json.Marshal and round-trips fine) and frontmatter only (the body is written raw). Found by the weekly fuzz sweep on FuzzPropertyValuesTypeZoo.'
priority: medium
why1: yaml.Unmarshal rejects yaml.Marshal's own output for a string beginning with a newline followed by content.
why2: The emitter writes a block-scalar header whose explicit indent indicator (4) disagrees with the indentation it then writes (2).
why3: 'It only miscounts when the first character is a newline: with content on the first line the indent is derived correctly, but a leading newline makes it compute the indicator from a line that has no content.'
why4: This is an upstream gopkg.in/yaml.v3 defect, not a rela bug. rela's part is that it hands arbitrary user strings to a library whose round-trip property it ASSUMED rather than verified -- and the assumption held for every value anyone had happened to try.
why5: A serialization boundary was treated as total. valueToNode had no notion that some values are unrepresentable in the target format, so there was nowhere for the question "what if this does not round-trip?" to be asked. The weekly fuzz sweep exists to find exactly where that assumption breaks.
prevention: 'Two concrete measures. (1) Any serialization seam that accepts arbitrary user values needs a ROUND-TRIP test -- marshal, unmarshal, COMPARE -- not a "no error on write" test. That distinction found the silent case here: "\n" never errored, it emitted |4+ and read back as "", which a write-only assertion would have called a pass. (2) When two backends serialize the same data differently, a value one accepts and the other refuses is a DIVERGENCE bug, and the fix direction must be chosen deliberately rather than by whichever side is easier to change: here fsstore was made to accept what pgstore already stored, while the sibling BUG-X7ICNM runs the other way (pgstore silently corrupts invalid UTF-8 that fsstore correctly refuses). Systemically: the duplicated valueToNode meant one fix left the other path broken, and the report predicted this. Shared serialization helpers belong in one package -- fsstore now delegates to internal/markdown rather than carrying a copy.'
status: done
---

## Description

`gopkg.in/yaml.v3` emits a block scalar it cannot read back. For a string whose
first character is a newline FOLLOWED BY CONTENT:

```go
raw, _ := yaml.Marshal([]string{"\n0"})
// raw == "- |4-\n  0\n"   <- indent indicator says 4, body is indented 2
yaml.Unmarshal(raw, &back)
// yaml: line 1: did not find expected '-' indicator
```

Found by the weekly fuzz sweep on `FuzzPropertyValuesTypeZoo`; reproduces in
~50s of `go test -fuzz`.

GitHub issue #993.

## Scope of the defect — narrower than it first appears

Two facts, both verified, that decide the fix:

**1. fsstore only. pgstore is NOT affected.** `pgstore.marshalProps`
(`internal/store/pgstore/entity.go:630`) uses `json.Marshal`, and JSON has no
block-scalar concept — the value round-trips exactly. Only fsstore serializes
property values as YAML.

**2. Frontmatter only. The body is NOT affected.** `formatEntity`
(`internal/store/fsstore/markdown.go:292`) writes `e.Content` raw beneath the
frontmatter fence; it never passes through YAML. Multi-line prose lives in the
body and is untouched. This is strictly about short frontmatter scalars.

So the blast radius is: a string property, containing a leading newline plus
content, on the fsstore backend.

**Widened in code review.** The shape set turned out larger than "leading
newline": a multi-line string starting with a tab breaks everywhere, and one
starting with a space breaks when a sequence sits anywhere above it. And the
string need not be top-level: `map[string]any{"v": "\n0"}` lost the newline
silently under the first fix, with no error, which is the shape the store fuzz
target generates. The fix now walks every container shape a property value can
take; the characterization lives in the `needsQuoting` godoc and REV-W5Z1KM
records the findings. Property KEYS go through the same emitter and had the
same defect (a key starting with a newline read back as `""`), and a key
yaml.v3 resolves to null (`~`) dropped the property outright; `KeyNode`
covers both.

## Decision: quote the scalar (option D)

Decided by the project owner.

The earlier inclination was to REJECT such values with a clear error, on the
reasoning that a leading newline in a frontmatter scalar is not content anyone
authors deliberately. **That reasoning is wrong**, and fact 1 is why:

Rejecting would make **fsstore and pgstore disagree about what a valid entity
is**. The same `PATCH` would succeed on Postgres and 400 on fsstore. A
storage-layer serialization limitation would become a data-validity rule, and
one that only some deployments enforce. That is a far worse property than
formatting churn.

It also violates the invariant the store layer should hold: persist what you are
given, or fail loudly — do not develop an opinion about which strings are worth
keeping. The value is legitimate data that one backend already stores fine.

## Approach

Force `Style: yaml.DoubleQuotedStyle` on string scalars that would otherwise
emit a breaking block scalar. Scope the check narrowly — only strings that
actually trip the emitter — so existing multi-line values keep their current
on-disk formatting and no reflow churn lands on unrelated files.

`valueToNode` (`markdown.go:172`) is the hook: it is four lines and already
wraps `node.Encode`.

## Two traps, both hit during diagnosis

- **`valueToNode` is DUPLICATED** in `internal/markdown/parser.go` and
`internal/store/fsstore/markdown.go`. Fixing one and re-running the fuzz target
still failed. Any fix must cover both, or entity writes and relation writes will
disagree.
- **The test must assert a ROUND TRIP** (write → read back → compare), not
merely "no error on write". A write-only assertion is exactly what would admit
the silent-data-loss variant: today the write FAILS, which is the safe outcome;
a naive fix that stops the error while still emitting unreadable YAML would turn
a loud failure into a corrupt file.

## The crashing input

Saved at `.ignored/issue-round/fuzz-993/98607a82b5bf052c`, deliberately NOT in
`testdata/fuzz/`. It belongs there — that turns the sweep's finding into a
regression test on every `go test` — but only WITH the fix. Alone it is a
deliberately-failing seed that would redden CI on every branch, so it ships
alongside.

## Acceptance

1. `"\n0"` as a string property round-trips through fsstore.
2. The same holds for the list-valued case the fuzzer found.
3. Existing multi-line property values keep their current on-disk formatting —
the fix does not reflow files it need not touch.
4. Both copies of `valueToNode` are fixed.
5. The committed fuzz seed passes as an ordinary test.
