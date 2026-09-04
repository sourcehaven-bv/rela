---
id: RR-CJKMAG
type: review-response
title: 'Minor and leverage findings: ragged rows, unknown palette keys, compat-sweep blind spot, fixture staleness'
finding: Four smaller findings from code review. (4) A row with more cells than Columns rendered as-is, widening that row past the rest and breaking the table grid; Part B made this script-reachable and a test explicitly blessed it. (5) ValidatePalette accepted unknown keys, so a typo like --dark-card-color for --dark-card-bg passed validation and was then silently dropped by colors(), leaving the operator with a default they had explicitly tried to override and nothing to explain it. (7) TestCompat_NoUnsupportedPropertyOnAnyElement skips properties absent from the dataset (shorthands like border-bottom), so a reader reasonably assumes every declaration is scored when several are not. (8) The vendored fixture had no staleness signal beyond the refresh recipe in a comment.
severity: minor
resolution: '(4) buildSection now normalizes each row to the header width — padding short rows, truncating long ones. The two directions are deliberately asymmetric: a missing cell leaves a blank in a grid that still aligns, while an extra cell breaks the columns for the whole table. Pinned by TestRender_RaggedRowsMatchHeaderWidth. (5) ValidatePalette now rejects unrecognized tokens and names the valid set; the token list is closed and small, so the diagnostic is free. Pinned by TestValidatePalette_RejectsUnknownKeys, which also asserts every key colors() reads validates. (7) The sweep now counts and names untracked properties in its log output (currently 20 scored, 7 untracked) so the coverage gap is visible rather than implicit. (8) A t.Logf note fires when the fixture exceeds 12 months, chosen as a prompt rather than a gate — a date-triggered CI failure would break builds for a reason nobody changed, which is the exact problem the pinned-not-fetched decision existed to avoid. Finding (6), the .gap spacer following only table sections, was confirmed intentional and is now documented as a deliberate asymmetry in template.go.'
status: addressed
---

## Findings and resolutions

Four smaller items from cranky-code-reviewer, grouped because none needed its
own decision. All were fixed rather than deferred — each was a few lines.

### 4. Ragged rows rendered a malformed table (minor)

A row with more or fewer cells than `Columns` was emitted as-is. Pre-existing in
`buildSection`, but Part B made it script-reachable and
`TestMailRender_TolerantOfOptionalShapes` had explicitly blessed `"ragged rows"`
as acceptable.

Fixed by normalizing to the header width. The two directions get different
treatment on purpose, and the code says why: a **short** row leaves a blank in a
grid that still lines up, while a **long** row widens itself past every other
row and breaks the column alignment of the entire table. So short rows are
padded and long rows truncated. A caller handing over a mismatched row has a bug
either way — but a misshapen table is a worse way to learn that than an empty
cell. Pinned by `TestRender_RaggedRowsMatchHeaderWidth` (short, exact, long, and
empty rows, all asserted to two cells).

### 5. Unknown palette keys passed validation (minor)

`ValidatePalette` checked values but not names, so `--dark-card-color` (real
key: `--dark-card-bg`) validated fine and was then silently ignored by
`colors()`. Nothing unsafe — values are still colour-checked — but an operator
saw the default they had explicitly tried to override, with no diagnostic
anywhere.

Now rejected, with the valid token set named in the error. The set is closed and
small, so listing it costs nothing. `TestValidatePalette_RejectsUnknownKeys`
covers both directions: a typo is refused, and every key `colors()` actually
reads is accepted — so this check cannot drift into rejecting a legitimate
override.

### 7. The compat sweep was weaker than it looked (leverage)

`TestCompat_NoUnsupportedPropertyOnAnyElement` skipped any property with no
dataset entry — shorthands like `border-bottom`, plus `color` and `font-family`.
Combined with its deliberately conservative "unsupported in *every* client" bar,
it could only ever fire on a property both tracked and universally dead. That is
a defensible floor and the comment was honest about the bar, but a reader would
reasonably assume every emitted declaration was checked.

The sweep now reports its own coverage:

```
scored 20 properties against the dataset; 7 untracked
(-ms-text-size-adjust, -webkit-text-size-adjust, border-bottom, color,
 font-family, font-style, padding-left)
```

The gap is now visible in test output rather than buried in a `continue`.

### 8. No staleness signal on the fixture (leverage)

A `t.Logf` note now fires when the dataset exceeds 12 months, pointing at the
refresh recipe in the file header.

Deliberately a **log, not a gate**. A date-triggered failure would break builds
for a reason nobody changed — precisely the "fails in CI for reasons unrelated
to the code" problem that justified pinning the fixture instead of fetching it.
Twelve months is long enough that a healthy repo never sees the note, short
enough that a genuinely abandoned fixture says so.

### 6. `.gap` follows only table sections (minor — no code change)

Confirmed intentional: a prose or empty-note section takes its separation from
the next `.sect-title`'s own top padding, so adding a spacer there would double
it. The reviewer's point was that the doc comment said "vertical gaps are spacer
ROWS" without noting the exception, leaving a future editor to wonder. The
asymmetry is now written down in `template.go`.
