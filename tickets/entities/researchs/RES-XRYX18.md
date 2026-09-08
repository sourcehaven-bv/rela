---
id: RES-XRYX18
type: research
title: How should entity commenting anchor to properties and text ranges in rendered markdown?
summary: 'Depend on github.com/vloothuis/textanchor v0.1.0 (MIT, extracted from margin: anchor engine + quotefind rendered->source mapping, a superset of the Hypothesis cascade with structural context and uniqueness scoring); normalise via FormatMarkdown to survive fsstore''s 80-col reflow, pin ContentHash for backend-independent staleness detection; ship property/section anchors first; add three-tier confidence and bound the substring search.'
status: done
---

## Problem

Users of the data-entry app want to comment on an entity: either on a named
property, or on a selection of the rendered markdown body. Two questions need
answering before implementation:

1. **How is a comment anchored** to a point in the entity, such that the anchor
survives ordinary editing of that entity — and what happens when it cannot?
2. **Where does commenting live** — an operator-declared schema pattern, a
built-in feature, or a built-in mechanism with operator-configured policy?

This matters now because the anchoring decision is load-bearing for the storage
schema. Getting it wrong means either re-migrating stored anchors later, or
shipping a feature whose comments silently detach from the text they discuss.

## Context

### Storage and write path (verified)

- Entities are markdown files with YAML frontmatter. `entity.Entity` is
`{ID, Type, Properties, Content, UpdatedAt}` — `Content` is the body
(`internal/entity/entity.go:54`).
- `POST /api/v1/{plural}` accepts **inline `relations`**
(`internal/dataentry/write_handler.go:227`), so a comment entity and its edge
are created in one request — no orphan window.
- The view engine already traverses relations into ACL-filtered collections
(`internal/dataentry/views.go:66`), so comments can ride the existing
`/api/v1/_views/` response with correct row-gating and no new endpoint.
- Entity types are entirely schema-driven; the repo's own
`review-response` type (`tickets/schema.yaml:694`) is already a comment-shaped
record, attached via `has-review-response` with an `inverse:`.

### The anchoring constraints (the crux)

Three findings, each independently fatal to naive offset anchoring:

1. **fsstore reflows every body on write**, wrapping paragraphs at 80 columns
(`internal/store/fsstore/markdown.go:188` → `markdown.FormatMarkdown`,
`internal/markdown/format.go:34`, `DefaultLineWidth = 80`). pgstore stores the
body raw. So *the same logical content has different byte offsets on different
backends*, and an offset can shift with no user edit at all.
2. **Markdown is rendered client-side** (`frontend/src/utils/markdown.ts:58`,
marked → DOMPurify). DOMPurify runs last with a strict attribute allowlist
(`ADD_ATTR: ['data-cb-idx']`, `markdown.ts:112`) — any new anchoring attribute
must be explicitly allowlisted or it is silently stripped.
3. **The rendered DOM is mutated after render**: mermaid and PlantUML replace
subtrees via `innerHTML` in a `flush: 'post'` watcher
(`EntityDetail.vue:222-231`). DOM-coordinate anchors are invalidated by async
post-processing.

Together these mean anchors must live in a **normalized source-markdown
coordinate space**, not in raw byte offsets and not in DOM coordinates.

### What already exists to build on

- **`data-cb-idx`** — the checkbox toggle maps a render-time ordinal back to a
source line (`markdown.ts:99`, `checkboxToggle.ts:31`, `EntityDetail.vue:281`).
This is the existing render↔source contract and the precedent to extend;
`checkboxToggle.ts:7` documents the invariant that renderer and mapper must not
disagree.
- **Server-side goldmark AST walking on the view path** — `collectMentions`
parses entity markdown and reads `t.Segment.Value(source)`
(`internal/dataentry/mentions.go:150-190`), i.e. **the server can already map
parsed nodes to exact source offsets**, using a parser explicitly configured to
match what the SPA renders (`mentions.go:195`).
- **`canonical.HashEntity`** (`internal/canonical/canonical.go:63`) — a
backend-stable content hash that normalizes through `FormatMarkdown` so
fsstore's reflowed body and pgstore's raw body converge. A ready-made
fingerprint for "has this changed since the comment was written", available on
**all** backends (unlike version history, which is postgres-only). Caveat: it
covers id + type + properties + body, so a property-only edit also changes it; a
body-only fingerprint would hash `FormatMarkdown(content)` directly.
- **`lineDiff`** (`frontend/src/utils/lineDiff.ts`) — an LCS line diff already
in-tree, with a documented size guard, reusable for re-anchoring.
- **Playwright e2e** including `checkboxes.spec.ts` — the precedent for testing
render↔source interaction in a real browser.

### Policy and security constraints

- **Validation policy (DEC-HWZHA):** a stale anchor is a *soft* condition — a
hand-editor can trivially produce it — so it must warn, never 422 or block a
save (`internal/dataentry/CLAUDE.md`).
- **Authorship must be stamped server-side.** The automation template's
`{{user.name}}` is the **git config user**
(`internal/automation/template.go:48`), not the request principal — a trap. The
automation engine has no principal access at all. `rela.principal` *is* exposed
to Lua as a frozen table (`internal/lua/runtime.go:976`), which is the
documented route (`docs/lua-scripting.md:414`).
- **Row-level ownership is not expressible today.** `RoleDef.Create/Update/Delete`
are plain type-name allowlists (`internal/acl/authz_write.go:138`); the `when:`
predicate exists only on *field* and *relation* grants. The predicate engine
already evaluates `current_user` and ships
`internal/predicate/testdata/accept/03_owner_check.lua` — literally
`entity.created_by == current_user.id` — so "edit only your own comment" is
plumbing, but it is a real ACL change needing its own security review.
- **Body content is not redactable today** (`internal/visibility/policyreader.go:199`,
`TODO(body-redaction)`). So a stored quote leaks nothing a reader of the target
couldn't already see — *provided* comments are gated by the same read verdict as
their target. If body redaction ever ships (`InaccessibleFieldContent` is
already reserved), stored quotes become an unredacted copy of hidden content.
This must be recorded as a forward-looking risk.
- **SSE is type-granular** — the wire payload is `{"type": "..."}` with no id
(`internal/dataentry/watcher.go:450`). Comments go live for free, but every open
view refetches on any comment write. Per-id events were considered and
deliberately rejected (TKT-POT9GQ); don't reopen that casually.
- On the fs backend (the default build) each comment is **a committed file** in
`entities/<type>/`, so comment churn lands in git history.

### Implementation constraints for a top-level `comments:` block (verified)

- **The loader has a hand-maintained allowlist.** `validTopLevelKeys`
(`internal/metamodel/loader.go:16`) must gain a `comments` entry or the loader
rejects the block it was built for. This exact drift already happened once with
`attachments:` (BUG-5XIN07); `TestValidTopLevelKeysMatchStruct`
(`loader_test.go:931`) is the reflection guard that now catches it.
- **`transforms:` is the better precedent than `attachments:`** — it validates at
load (`loader.go:243` → `internal/metamodel/transforms.go:26`), sorts keys for
deterministic errors, and *canonicalizes* defaults so no downstream consumer
re-defaults. `attachments:` has no load-time validation at all.
- **Metamodel config must be read live, never captured.** `watcher.go:307`
swaps the metamodel atomically on file change; a captured pointer or a registry
built once at construction goes stale.
- **`Metamodel` carries a plimsoll grandfather directive** (`types.go:18`), so
new accessors belong on a narrow `CommentPolicy`-style view type, following
`NewAttachmentPolicy` (`attachments.go:85`).
- **Includes are a gap to decide explicitly:** neither `attachments:` nor
`transforms:` is legal in an included file (`include.go:13`), and an include
declaring one is *silently dropped*.
- **Config must not ride `/_config`.** That endpoint is served verbatim and must
not be filtered per principal (TKT-M1AX6P, pinned by
`TestNavPermission_ConfigUnfiltered`). Comments get their own endpoint — the
`/_transforms` precedent (`api_v1.go:113`).
- **A new write verb is a known-hard extension point.** It touches `acl.Op` plus
four closed switches, `translateVerb` (`affordances.go:42`, which *panics* on
unknown verbs by design), `perItemVerbs`, docs, and the SPA.
`dataentry/CLAUDE.md` already defers `transition:*` / `relation:*` for exactly
this reason. Note `lint_test.go`'s grep guard only walks the **package root**,
so comment write code in a sub-package would silently escape it.
- **New-route test probes are mandatory:** `router_walk_test.go` and
`readonly_write_route_invariant_test.go`.
- **Coverage:** default package floor 50 (`.testcoverage.yml:18`); 55 inside
`dataentry`, 65 inside `metamodel`. Security-boundary packages are pinned higher
by convention (visibility 85), so expect review pressure there.
- **Frontend:** vitest with happy-dom by default, but `markdown.test.ts` opts
into jsdom because **happy-dom mis-serializes adjacent block elements under
DOMPurify** (BUG-SQSV6V). Anything walking rendered markdown must do the same.
`noNetwork.test.ts` (BUG-762I34) forbids `onMounted` fetches reaching the
network — the exact shape a comments panel takes.

### A built-in `comment` entity type would be unprecedented

Nothing in rela reserves an **entity type name**. The only reserved-name check
at load applies to custom *scalar* types (`loader.go:147`, over `m.Types`);
`validateEntityStructure` has no entity-name check at all. Reserved namespaces
in rela are the underscore URL segment (`_config`, `_search`) and the `system:`
principal prefix — not type names. So shipping a built-in `comment` type would
collide with any existing project that already declares one, and adding a
reservation check is a breaking change.

### Version pinning is postgres-only — but staleness detection is not

- `versionServiceFor` returns a genuinely-nil service in every non-postgres build
(`internal/appbuild/versionsweep_nosweep.go:19`); fsstore's substitute is git,
sqlite has none. The documented degradation pattern is a plainly-stated 501
capability gap (`history_handler.go:44`), not a silent fallback.
- Pin on **`ContentHash`, never the `Version` ordinal** — ordinals are computed
at read time and are purge-fragile, and rela permits id reuse so `(entity_id,
version)` can cross into an unrelated reclaimed entity
(`pgstore/version.go:81`).
- Crucially, `contentHashOf` is just `canonical.HashEntity`
(`pgstore/version.go:70`), explicitly "matching the live entity hash". That
function is backend-independent — so **"is this comment stale?" works on every
backend** even though *retrieving the old version* is postgres-only. This
decouples staleness detection from version history.
- There is no `GetVersionByContentHash` today; resolving a hash-pinned anchor to
a snapshot would need a new store method plus `storetest` conformance.
- Do **not** add comment config to `RenderProjection` — its hash stability is a
versioning contract (`shapeprojection.go:19`: "This is a SIBLING … Do not merge
them").

### Library landscape for text anchoring (surveyed)

Only three projects in this ecosystem are alive; the entire `dom-anchor-*` /
`annotator.js` / Apache Annotator lineage is dormant, archived, or formally
retired.

| Project | Licence | State |
|---|---|---|
| `hypothesis/client` | BSD-2-Clause | active; vendored anchoring |
| `@recogito/text-annotator` | BSD-3-Clause | active; the live successor, quote + start/end, W3C-aligned |
| `approx-string-match` | MIT | stable/complete; the matching primitive |
| `dom-anchor-text-quote` | — | unmaintained; depends on a third-party `diff-match-patch` fork whose Google upstream is archived (2019) |
| `annotator.js` | **MIT OR GPL-3.0** | dead since 2015; if adopted, the MIT election must be explicit |
| `anchorpoint` (Python) | "Free To Use But Restricted" | **not permissive — unusable** |
| `robertknight/anchor-quote` | none | archived, no licence — unusable |

**There is no Go library for W3C selector anchoring** — a Go implementation is a
from-scratch port (the Myers bit-parallel matcher plus a scoring cascade),
though both reference implementations are permissively licensed for reading.

Two in-tree facts matter here:

- **`sergi/go-diff`** (the Go diff-match-patch port) is **already in `go.mod`**
(line 157, currently indirect) — the fuzzy-matching primitive is effectively
available without a new direct dependency decision.
- **rela's existing `fuzzy` is not a locator.** `internal/filter/fuzzy.go:6`
implements set-based **trigram Jaccard similarity** (order-insensitive,
position-free). It can *score* a candidate window but cannot *find* the best
window in a document. Locating a moved quote needs a different algorithm than
anything currently in-tree — do not assume `fuzzy()` can be reused for it.

### Prior art in-house: `~/Work/margin` (decisive)

`margin` is the user's own MIT-licensed Go tool that solves *exactly* this
problem — annotating markdown without modifying the source, anchored by fuzzy
matching so annotations survive edits. It has a working, tested `anchor/`
package (11 tests, passing) and has been in real use since 2025-12. This is a
reference implementation, not a sketch, and it supersedes the "no Go library
exists" conclusion: one exists, it is ours, and it is adoptable.

**`anchor.Anchor` stores five descriptors** (`anchor/anchor.go`), a superset of
the W3C/Hypothesis selector set:

| Field | Purpose |
|---|---|
| `Quote` | the exact selected text |
| `Prefix` / `Suffix` | ~50 chars either side, for disambiguation |
| `ContainingSentence` | full sentence, captured **only for quotes < 50 chars** |
| `HeadingContext` | nearest preceding ATX heading |
| `ParagraphIndex` | 0-based paragraph index within that section |

The last three are **beyond** what Hypothesis stores, and they directly address
its two documented weaknesses. `ContainingSentence` is the answer to the
short-generic-quote pathology (#3919); `HeadingContext` + `ParagraphIndex` give
a *structural* fallback that survives whole-paragraph rewrites where quote
matching alone would orphan.

**Resolution** (`anchor/resolve.go`) is a two-phase candidate search — all exact
quote occurrences first, fuzzy matching only if none — then weighted scoring:

```
0.40*quote + 0.20*prefix + 0.20*suffix + 0.10*structural + 0.10*uniqueness
```

with `MinConfidence` 0.5, a 0.8 penalty on fuzzy-derived candidates, and
structural score itself `0.7*heading + 0.3*paragraph`. Note the **uniqueness
term (1/occurrences)** — a component absent from Hypothesis's weights that
directly scores the W3C §4.2.4 ambiguity problem rather than merely mitigating
it. `ResolveAll` hoists heading/paragraph extraction across anchors so a
document's whole annotation set resolves in one pass.

**Matching** (`anchor/fuzzy.go`) is length-adaptive: Levenshtein ratio
(`agnivade/levenshtein`) under 100 chars, token-Jaccard above; a sliding window
at ±25% of quote length finds the best substring; queries under 5 chars are
refused outright — the minimum-quote-length guard my research recommended,
already implemented.

**`internal/web/quotefind.go` solves the render↔source problem I had flagged as
unsolved.** `extractRenderedTextWithMapping` walks the goldmark AST and builds a
**per-byte `posMap` from rendered text back to source offsets**, handling soft
line breaks and block separators. This is the technique rela *has available*
(`mentions.go:150` already reads `t.Segment`) but has never implemented. It
exists precisely because a browser selection yields rendered text without
markdown syntax — the same mismatch rela's SPA would hit.

**What this changes:**

- The Stage-2 matcher is **adoption, not a from-scratch Myers port**. The
recommendation to reimplement `match-quote.ts` in Go is superseded.
- `margin`'s design is a **strict superset** of the Hypothesis cascade, with
structural context and a uniqueness term it lacks.
- Storage shape is already proven git-friendly (one TOML file per annotation,
mirroring document paths) — conceptually identical to rela's one-markdown-
file-per-entity model, which de-risks the "comments as committed files" concern.

**Gaps to close on adoption** (none fundamental):

- **No `FormatMarkdown` normalisation.** margin anchors into raw document text;
rela must normalise both at store and at match time or fsstore's 80-column
reflow will orphan anchors. This remains the highest-leverage decision.
- **Binary orphaned/resolved, not three-tier.** `ResolveResult` has `Orphaned` +
`Confidence` + `OrphanReason`, so the data for a middle "orphan-with-
suggestion" band is already present — but the tier logic is not. The
MSR-TR-2001-107 finding still needs applying.
- **`bestSubstringMatch` is O(windows x levenshtein)** over a paragraph. Fine
for margin's scale; needs a bound before it runs per-comment on a server request
path.
- Anchors are byte offsets into a `string`; rela is UTF-8 throughout, so
multi-byte handling needs checking at the boundary.

## Options

Prior art settles the *shape* of the answer: W3C Web Annotation (Rec. 2017)
defines the selector vocabulary, Hypothesis is the reference re-anchoring
implementation, GitHub demonstrates version-pinning, and MSR-TR-2001-107 is the
one study that measured user expectations. The live question is which regime
rela adopts.

**The architectural fork.** Anchoring splits into two regimes, and the choice is
structural, not a tuning parameter:

- **Positional / CRDT anchors** (Google Docs, Yjs, Peritext) — the anchor is an
*identity*, and the system participates in **every** edit.
- **Re-anchoring** (W3C selectors, Hypothesis) — the anchor *describes* content,
and the system sees only before-and-after.

**Regime 1 is unavailable to rela in principle.** Entity bodies are edited by
external tools, git, and other processes; the storage philosophy in `CLAUDE.md`
guarantees whole-file writes rela never observes. Positional anchors require
mediating every edit, and that precondition fails. This is a fact about rela's
design, not a preference — so the options below all sit inside the re-anchoring
regime.

### Option A — Property/section anchors only

Anchor to a property name or the operator-authored `sectionId`.

- **Pros:** Zero drift — anchors are *names*, not offsets. No new render
machinery, no matching algorithm, no orphan state. Ships as a schema pattern.
- **Cons:** Cannot comment on a sentence, which is the stated requirement.
- **Effort:** Small.

### Option B — Quote-only anchors

Store `exact` plus prefix/suffix; locate by search on render.

- **Pros:** Survives insertion-before, reordering, and (critically for rela)
the fsstore 80-column reflow, provided text is normalised first.
- **Cons:** W3C §4.2.4 concedes quote matching is **not guaranteed unique** —
duplicate text anchors ambiguously. No fast path: every comment searches the
document on every render. Hypothesis #3919 documents >10s main-thread blocking
for short quotes in long documents.
- **Effort:** Medium.

### Option C — Quote + position + context (the Hypothesis cascade)

Store all three selectors. Try position first as a *hint*, then assert the
resolved text equals the stored quote; fall back to fuzzy search.

- **Pros:** The settled belt-and-braces design. Position gives speed, quote gives
truth. Hypothesis's `maybeAssertQuote` **rejects a positional hit whose text
does not match** — so a shifted offset degrades to a search rather than silently
anchoring the wrong text. Its weighted score (quote 50, prefix 20, suffix 20,
position 2 — position explicitly a tie-breaker) is a directly reusable starting
point, as is the error budget `min(256, len(quote)/2)`.
- **Cons:** Needs an approximate-substring locator, which rela does **not** have
(`internal/filter/fuzzy.go` is order-insensitive trigram Jaccard — a scorer, not
a locator). Real orphan rates in the wild are 22–27% (Aturban et al.,
arXiv:1512.06195), though that measures uncontrolled public web pages and is an
upper bound, not a prediction for a controlled corpus.
- **Effort:** Medium-large.

### Option D — Version-pinned anchors (GitHub's model)

Pin each comment to the content it was written against; re-anchor onto the live
body best-effort; when that fails, display the comment against its pinned
version rather than orphaning it.

- **Pros:** The pin is **immutable and always resolvable**, so a failed
re-anchor still renders meaningfully — strictly better than an orphan list. This
is exactly what GitHub does: `original_commit_id` + `original_line` +
`diff_hunk` are retained verbatim, and `diff_hunk` is effectively a stored
`TextQuoteSelector`. W3C §4.3 recommends the same via `TimeState`/`cached`.
- **Cons:** Retrieving a *pinned snapshot* is postgres-only in rela. Pinning must
key on `ContentHash`, never the `Version` ordinal (read-time computed,
purge-fragile, and id-reuse can cross lineages).
- **Effort:** Medium on top of C; larger if snapshot retrieval is required on
every backend.

### Option E — Positional / CRDT anchors

Rejected above: rela does not observe the edits. Recorded only so the rejection
is on the record. Worth noting that on a whole-document rewrite CRDT anchors
survive *mechanically* to a location whose meaning has changed, flagging nothing
— arguably worse than an honest orphan.

## Recommendation

**Option C as the mechanism, with Option D's pin as a stored field and Option A
shipped first.** Concretely, a three-stage delivery:

**Stage 1 — property/section anchors (Option A).** Delivers commenting end to
end with zero drift risk, and builds the record type, the ACL story, the panel
UI, and server-side author stamping. Everything here is reusable by later
stages.

**Stage 2 — text ranges via the Hypothesis cascade (Option C).** Store `exact` +
prefix/suffix + a position hint + a body fingerprint. Resolve position-first,
**assert the quote**, fall back to approximate search. Two rela-specific
requirements fall out of the constraints already recorded:

- **Normalise through `markdown.FormatMarkdown` before storing and before
matching.** This is what makes the fsstore 80-column reflow a no-op rather than
a mass-orphaning event, and it is also W3C's own requirement for
`TextQuoteSelector`. It is the single highest-leverage decision in the design.
- **Anchor in source-markdown coordinates, not DOM coordinates** — DOMPurify's
attribute allowlist and the post-render mermaid/PlantUML `innerHTML` rewrites
make DOM offsets untrustworthy.

**Stage 3 — pinning (Option D).** Store `canonical.HashEntity` (or a body-only
`FormatMarkdown` hash) at comment creation. Because `contentHashOf` is just
`canonical.HashEntity`, **staleness detection works on every backend** even
though snapshot *retrieval* is postgres-only — so "this comment was written
against different text" is universally available, and pg additionally offers
"show me that text". Degrade per the `history_handler.go:44` precedent: a
plainly-stated capability gap, never a silent fallback.

### Where to implement the matcher — adopt `margin/anchor`

Server-side in Go, by depending on **`github.com/vloothuis/textanchor`**
(v0.1.0, MIT, public) rather than writing a matcher from scratch. The `anchor`
and `quotefind` packages were extracted from `margin` into that library so rela
consumes them as a normal dependency instead of copying code between repos.
`textanchor.New`/`Resolve`/`ResolveAll` are the anchor engine;
`quotefind.Find`/`FindWithContext`/`RenderedTextWithMapping` map a selection
over rendered markdown back to source offsets. The `Anchor` struct carries both
TOML and JSON tags, so it serialises straight into rela's wire types. This
supersedes the earlier plan to port Myers bit-parallel matching from
`match-quote.ts`: a working, tested Go implementation already exists, and its
descriptor set (adding `ContainingSentence`, `HeadingContext`, `ParagraphIndex`,
and a uniqueness score) is a **strict superset** of the Hypothesis cascade,
addressing two of Hypothesis's own documented weaknesses.

Port `internal/web/quotefind.go` alongside it: its
`extractRenderedTextWithMapping` builds a per-byte rendered→source `posMap` from
the goldmark AST, which is the missing piece for turning a browser selection
over rendered HTML into source-markdown coordinates. rela already has the same
primitive available (`mentions.go:150` reads `t.Segment`) but has never used it
this way.

Rejecting the client-side alternative (`@recogito/text-annotator`) still stands,
and more strongly now: anchoring belongs on the server side of the sanitiser,
and the Go code already exists.

**Required adaptations, in priority order:**

1. **Normalise through `markdown.FormatMarkdown`** at store and match time.
margin anchors into raw text; without this, fsstore's 80-column reflow orphans
anchors wholesale. Highest-leverage change.
2. **Add the third confidence tier.** `ResolveResult` already carries
`Confidence` and `OrphanReason`, so the data exists; only the place-silently /
suggest / orphan-outright banding is missing.
3. **Bound `bestSubstringMatch`.** It is O(windows x levenshtein) per paragraph
— acceptable for a CLI, not for a per-request server path with many comments.
4. **Verify UTF-8 boundaries.** Anchors are byte offsets; confirm multi-byte
handling at the API edge.

### Non-negotiables carried from the evidence

1. **Three-tier confidence, not binary.** MSR-TR-2001-107 found that when a
candidate matched only one keyword, median user rating was **1.0/7
("terrible")** — *a bad guess is worse than no guess*. Place silently above a
high threshold; orphan-with-suggestion in the middle band; orphan with **no**
guess below it. Start conservative.
2. **Never hide a failed anchor.** Hypothesis originally hid orphans and users
believed their annotations had vanished; v1.2.0 added an Orphans tab. GitHub
collapses-and-labels, never deletes. Demote and label, and always show the
stored quote as context.
3. **"Failed to anchor" is an explicit state, never an absence** (Hypothesis
   #954's silent third state is a real bug class) — and per DEC-HWZHA it is a
**warning, never a 422**.
4. **Guard pathological search:** enforce a minimum quote length and always pass
the position hint (Hypothesis #3919).
5. **Comments use *after*-anchors and must not grow at boundaries** (Peritext).

### Tradeoffs accepted

- Some anchors will orphan. 22–27% is the uncontrolled-web upper bound; a
controlled corpus with normalisation should do materially better, but the design
must treat orphaning as normal rather than exceptional.
- Ambiguous duplicate text may anchor to the wrong instance; prefix/suffix and
the position tie-breaker reduce this without eliminating it (W3C §4.2.4).
- Snapshot display is postgres-only.

### Deliberately deferred

- **Row-level "edit only your own comment"** needs `When` on
`RoleDef.Create/Update/Delete`. The predicate engine already evaluates
`current_user` and ships `03_owner_check.lua`, so this is plumbing — but it is a
real ACL change and belongs in its own ticket with its own security review.
- **A built-in `comment` entity type** — nothing in rela reserves an entity type
name, so this would collide with existing projects. Prefer rela owning the
*mechanism* (anchoring, stamping, UI) over rela owning the *type*.
- **Per-id SSE events** — deliberately rejected (TKT-POT9GQ); do not reopen as
part of this work.

### Forward-looking risk to record

If body-level redaction ever ships (`InaccessibleFieldContent` is already
reserved, `visibility/policyreader.go:199`), every stored quote becomes an
unredacted copy of potentially-hidden body text. Today this is safe because
bodies are not policy-hideable and comments inherit their target's read verdict.
It must be re-examined the moment that TODO is actioned.
