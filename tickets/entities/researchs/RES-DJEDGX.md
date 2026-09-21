---
id: RES-DJEDGX
type: research
title: How should the @ mention menu rank candidates across type, id and title?
summary: 'Recommends @leeoniya/ufuzzy (4.2 KB, MIT) for the @ mention menu''s client-side ranking, over a concatenated haystack, with an explicit tunable quality floor. It was the only one of six libraries that accepts a hyphenated query as typed, because it splits the NEEDLE on punctuation; fzf''s JS port and fuzzysort both return zero hits for that form and need pre-normalisation. Also documents fzf''s actual FuzzyMatchV2 scoring constants and tiebreak rules, and measures performance (uFuzzy is near-flat to 50k candidates thanks to a regex prefilter; below ~1000 candidates any library is imperceptible, and the @ menu is capped at 1000, so performance did not decide the choice). Caveat added after the design review: the terms-based quality floor recommended here was validated only on complete-word queries and rejects every PREFIX match, so it must not be used as written — see RR-NNFWGP.'
status: done
---

Research for TKT-6MZ42J. All library behaviour below was measured against the
actual packages (Node 26, warmed, median of 7), not read off documentation.

## Problem

Rank `@` completion candidates so that one query matching *parts of different
fields* works: `fancy-some-word-in-title` must find the entity whose type is
`FancyReport` and whose title is "some word in title".

## Context: how fzf actually scores

`src/algo/algo.go`, `FuzzyMatchV2` — a Smith-Waterman variant that allows no
omission or mismatch (the pattern must be an in-order subsequence). O(nm) on a
match, O(n) on a miss.

| Constant | Value | Rewards |
|---|---|---|
| `scoreMatch` | 16 | each matched char |
| `scoreGapStart` | -3 | opening a gap |
| `scoreGapExtension` | -1 | extending a gap |
| `bonusBoundary` | 8 | match after a non-word char |
| `bonusNonWord` | 8 | match on a non-word char |
| `bonusCamel123` | 7 | `aB` camel or letter→digit transition |
| `bonusConsecutive` | 4 | floor for chars in a contiguous run |
| `bonusFirstCharMultiplier` | 2 | doubles the first matched char's bonus |

Scheme-dependent: `bonusBoundaryWhite` = 10, `bonusBoundaryDelimiter` = 9,
`delimiterChars = "/,:;|"`.

Two design points worth stealing regardless of library choice:

- A gap of roughly 8 characters cancels a boundary bonus. The doc comment cites
average word length — tuned so fzf stays a fuzzy finder rather than an acronym
finder.
- A consecutive run inherits `max(own bonus, 4, first-char-of-chunk bonus)`: a
run is valued by *how well it started*. That is what stops `foo-bar` beating
`foobar` on the query `foob`.

Tiebreak order is score first, then a lexicographically compared tuple from
`length` (the default), `chunk`, `pathname`, `begin`, `end`, `index`.

## Options

Sizes are minified + gzip. Note `fzf`'s shipped `dist/fzf.es.js` is
*unminified*, so its commonly cited 36KB/8.4KB is not its minified size.

| Lib | Ver | min+gzip | Boundary / camel | Multi-field | Positions | License | Maintenance |
|---|---|---|---|---|---|---|---|
| `@leeoniya/ufuzzy` | 1.0.19 | 4.2 KB | `interLft1/2` counters; camel used for splitting | concat idiom, no weights | yes (`info.ranges`) | MIT | active, most recent commits of the set |
| `fzf` (ajitid) | 0.5.2 | 6.1 KB | yes, exact fzf constants | `selector` → one string | yes | BSD-3 | dormant: no release in 3.4 years, last 5 commits all Dependabot |
| `fuzzysort` | 4.0.2 | 8.4 KB | boundary yes, no camel | `keys`, weights DIY via `scoreFn` | yes | MIT | active, v4 breaking |
| `fuse.js` | 7.5.0 | 9.6 KB | no (bitap) | yes, only real numeric weights | opt-in | Apache-2.0 | active |
| `@nozbe/microfuzz` | 1.0.0 | 1.5 KB | word-start only | `getText → string[]` | yes | MIT | feature-frozen (2023) |
| `match-sorter` | 8.3.0 | 3.5 KB | 7 ordinal tiers only | `keys`, ordinal only | **no** | MIT | active |

`fzf`'s 2.67M weekly downloads is a name collision with the Go CLI, not
adoption. VS Code's `filters.ts` `fuzzyScore` is extractable (MIT, 981 lines)
but unpublished, non-reentrant and `_maxLen`-capped at 128. `command-score` is
archived.

## Measured performance

Haystack of `"<Type> <five-word title>"`:

| N | `fzf` JS | uFuzzy |
|---|---|---|
| 1,000 | 2.2 ms | 0.26 ms |
| 5,000 | 12.3 ms | 0.55 ms |
| 20,000 | 34.2 ms | 2.00 ms |
| 50,000 | 79.9 ms | 1.89 ms |

fzf-js is linear with a real DP constant; uFuzzy is near-flat because a regex
prefilter rejects nearly everything before scoring. Below ~1,000 candidates any
library is imperceptible — and the `@` menu is capped at 1,000 by
`maxFreeTextSearchResults`, so **performance does not decide this choice**.

The widely cited "78× faster than fuse.js" figure is uFuzzy's author's own
2023-10 benchmark and predates the fuzzysort v4 rewrite; treat it as directional
only.

## Recommendation

**uFuzzy over a concatenated `"<Type> <title>"` haystack, type first.**

The decisive fact is that uFuzzy accepts the query *as the user types it*,
because it splits the **needle** on punctuation (`interSplit: "[^A-Za-z\d']+"`,
plus `intraSplit: "[a-z][A-Z]"` for camelCase):

```js
new uFuzzy().search(hay, 'fancy-some-word-in-title')
// → 'FancyReport some word in title'   (verified)
```

It treats `fancy some word in title`, `fancyreport-some` and `FancyReport some`
identically. **fzf, the fzf JS port and fuzzysort all return zero hits for the
hyphenated form** — they match `-` literally and need the query pre-normalised
first.

### Concatenated haystack vs per-field keys

Concatenation gives one pass, lets a query span the type/title seam naturally,
and earns a boundary bonus free at the field join. It costs per-field weighting,
and characters may be drawn from unrelated fields.

Per-field keys express "title outweighs type" but impose a structural limit,
measured on fuzzysort:

```
keys: ['type','title']
  "fancysome"          hits=0    ← a single token cannot span two fields
  "fancyreport title"  hits=1    ["FancyReport" 1.000, "some word in title" 0.918]
```

A **multi-word** query does compose across fields (per-part winner key, summed);
a **single unbroken token** cannot. Since the query normalises to spaces anyway,
fuzzysort is a viable second choice if per-field weighting later becomes a hard
requirement.

### One ranking caveat, verified

`RiskItem fancy some word in title` **outranks** `FancyReport some word in
title`, because `fancy` matches a whole term in the former's title. If a
type-prefix hit must dominate, add a start-offset boost — uFuzzy's `sort` is a
plain comparator you own, and `info.start` is provided.

## Quality floor: showing "No matches" instead of noise

> **Correction (design review, RR-NNFWGP).** The `info.terms >= nTerms` floor
> recommended in this section is WRONG as written and must not be implemented.
> It was validated only on complete-word queries. `terms` counts *exactly*
> matched terms — both edges on a word boundary — so a term matching a PREFIX
> contributes 0. Measured: `fanc` → terms=0, `fancy-som` → terms=1 of 2, and
> `TKT-AB` drops `TKT-ABCD`. In an autocomplete, where every keystroke is a new
> query, that blanks the menu until a word is completed. The rest of this
> section (why `interIns` is the wrong knob) still holds.

A genuinely unrelated query already returns nothing (`search(hay,
'zzz-nonexistent')` → `order = []`). The real risk is the *scattered* false
positive, where characters come from unrelated fields.

**The option knobs are the wrong tool.** Sweeping `interIns` (max chars between
terms) across 0-30 never separates the true hit from the junk:

```
default / interIns:10..30 → RiskItem, FancyReport, DecisionRecord(junk)
interIns:8                → RiskItem, FancyReport   (junk gone, but fragile)
interIns:4                → RiskItem                (correct answer LOST)
interIns:0                → no matches
interLft:2                → no effect on the junk
```

**Use a post-hoc threshold on `info.terms`** — run the three stages yourself and
require that every needle term matched a whole haystack term:

```js
const uf = new uFuzzy()
const nTerms = uf.split(q).length
const idxs = uf.filter(hay, q)
const info = uf.info(idxs, hay, q)
const order = uf.sort(info, hay, q)
const kept = order.filter((o) => info.terms[o] >= nTerms)
if (kept.length === 0) showNoMatches()
```

Verified on the same data with a 5-term query:

| candidate | `terms` | `interIns` | verdict |
|---|---|---|---|
| `RiskItem fancy some word in title` | 5 | 4 | keep |
| `FancyReport some word in title` | 5 | 10 | keep |
| `DecisionRecord fancy footwork…` | 1 | 22 | reject |

This keeps both correct answers and drops the junk, which no `interIns` value
achieved. Because uFuzzy exposes raw counters rather than a composite score, the
floor is explicit and tunable — the practical payoff of its "no black-box score"
design.
