---
id: PLAN-26SQ9B
type: planning-checklist
title: 'Planning: Extend the icon set to a curated ~150 names, generate registry + docs from one source, add icon: none'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** documented in full on TKT-EG33Y1 (In scope / Out of scope sections).
Summary: one canonical icon definition in Go generating the Go allowlist + the
SPA registry + a docs table; expand 16 → ~150 curated names; add `icon: none`;
reserve the sidebar icon column for no-icon items; CI drift check.

Explicitly OUT: all-1600-lucide passthrough, custom SVG upload, icons on new
surfaces, a group-level icons-off switch, sidebar restyling.

**Acceptance Criteria:** the nine criteria on TKT-EG33Y1. Each is mapped to a
concrete test in the Test Plan below.

## Research

- [x] For larger features: run `/research` — N/A, approach settled with the
requester up front (four design questions answered before planning); no
unfamiliar subsystem involved.
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — see above.

**Existing Solutions:**

*Libraries.* `lucide-vue-next` ^1.0.0 is already a dependency and stays. No new
dependency is needed: the generator is Go text/template over a Go table, run by
`go generate`, matching how the repo already generates artefacts.

*Prior art in this codebase — three precedents, all consulted:*

1. `internal/dataentry/apps_css.go:1-21` + `TestAppTokensCSSInSyncWithFrontend`
(`custom_shell_test.go:25`) — a **copy** of a frontend file embedded in Go, held
byte-identical by a test. Closest structural analogue, but it copies rather than
generates, and the doc comment says "do not hand-edit … edit the frontend source
and re-copy". We invert the direction (Go is canonical) and generate rather than
copy.
2. `internal/dataentryconfig/config.go:323-350` `ValidIconNames` + `:362-378`
`ValidCalendarColors` — the existing allowlist pattern, including the "exported
so the check and its parity test share one source" convention. The new generated
allowlist keeps the same exported name and shape so `validateIconName` is
untouched.
3. `internal/dataentryconfig/icons_test.go` `TestIconAllowlistMatchesFrontend` —
the current pinning mechanism. It **regex-parses TypeScript** (`iconKeyRe`) and
its own comment concedes the fragility ("a spread, nested literal, or aliased
import will do it"), which is why it needs a count check as a parse-regression
guard. Generation removes the need to parse TS at all; this test is replaced by
a regenerate-and-diff check.

*Docs generation prior art.* `just docs` / `just docs-check`
(`scripts/generate-docs.sh`) generates `docs/` from the `docs-project` rela
graph and CI fails on `git diff --exit-code docs/`. That is a *different*
generator (rela entities → markdown), so the icon table cannot ride on it
directly, but **`docs-check`'s diff gate is exactly the mechanism we want** and
`just ci` already runs it. Plan: a separate `just generate-icons` writing into a
marked region of `docs/data-entry.md`, with the drift gate extended to cover it.

*Prior rela tickets.* TKT-8GUI60 (done) introduced the icon set. Two of its
review-responses are load-bearing here:
- **RR-GTOQCF** (addressed, minor) found the docs' prose copy of the names went
stale within one ticket and removed it, leaving the startup error message as the
only reference. Its resolution explicitly frames the omission as deliberate
"rather than an oversight the next person should 'helpfully' fill in". **This
ticket is allowed to fill it in only because the table is generated** — the
objection was to an unpinned copy, not to documenting names. The generated
region must say so in a comment.
- **RR-VESDBZ** (wont-fix) noted kanban's `v-if="column.icon"` makes the
fallback unreachable, so kanban and sidebar disagree on unknown input. That
disagreement is directly adjacent to `icon: none` and is re-examined below.

*Reference implementation for the no-icon rule.* Apple HIG "Menus → Icons"
(linked from the ticket): use icons sparingly and with purpose. Corroborating
prior art in this repo: kanban columns already treat a *missing* icon as "no
icon" (`v-if="column.icon"`), so no-icon rendering is not a new concept — it is
simply unreachable on nav, where an icon is always derived.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach**

*1. Canonical table (new, hand-edited).* `internal/dataentryconfig/icondefs/icondefs.go`
holds an ordered `[]IconDef{Name, Lucide, Category, Desc, Chrome}` slice.

**A leaf package, deliberately** (RR-8VBAJM). The generator must import the table
and also *write into* `internal/dataentryconfig`. If both lived in one package, a
hand-edit that broke the generated file would make the generator itself
unbuildable — a bootstrap deadlock escapable only by hand-reverting. `icondefs`
contains no generated file, so it always compiles, so the generator always runs.

`Chrome bool` marks the entries the SPA references directly (RR-Y1PO6R). Ordered, not a
map, so generated output is deterministic without a sort and categories keep
their authored order in the docs table. Go rather than YAML: no parser, no
embed, and a nonexistent-component typo is caught by the generated TS failing to
compile rather than at runtime.

*2. Generator (new).* `cmd/gen-icons/main.go`, invoked by a `just generate-icons`
recipe (**one** invocation path, not also `go:generate` — RR-8VBAJM). It emits
three artefacts from the one slice:
- `internal/dataentryconfig/icons_gen.go` — `ValidIconNames map[string]bool`,
keeping the exported name so `validateIconName` (`config.go:388`) is unchanged.
- `frontend/src/utils/icons.ts` — the named imports and the `ICONS` map. The
hand-written parts (`DEFAULT_ICON`, `resolveIcon`, `isKnownIcon`, `iconNames`
and their doc comments, which carry the security rationale) stay in a separate
hand-written module; the generated file exports only the map. This keeps the
security-critical lookup logic under human review and out of a template.
- `docs-project/entities/guides/GUIDE-data-entry.md` — a categorised table
inside `<!-- BEGIN generated: icons -->` / `<!-- END generated: icons -->`.

**Not `docs/data-entry.md`.** That file is itself auto-generated (its first line
says so) from the guide entity above, by `scripts/generate-docs.lua` via `just
docs`. Writing the table into `docs/` directly would be silently reverted on the
next `just docs`, and `just docs-check` — which is in `just ci` — would then
fail on an unrelated PR. Verified during planning: the generator copies
`entity.content` verbatim (`generate-docs.lua:16-22`), so an
HTML-comment-delimited region passes through to `docs/data-entry.md` intact. The
pipeline is therefore:

canonical Go table → (icon generator) → GUIDE-data-entry.md region → (`just
docs`) → docs/data-entry.md

*3. Deprecated-alias correction (in scope, discovered during planning).* Lucide
v1.0.0 still exports legacy aliases but they are absent from its `.d.ts`
`declare const` list. Three names the current registry imports are aliases:

| current import | canonical v1 name |
| --- | --- |
| `Home` | `House` |
| `AlertTriangle` | `TriangleAlert` |
| `CheckCircle2` | `CircleCheck` |

Verified by component identity against the installed package. The canonical
table uses the canonical component names throughout; the **config-facing names**
(`dashboard`, `warning`, `done`) are unchanged, so this is invisible to authors
and satisfies AC 8 while removing a latent break at the next lucide major.

*4. `icon: none`.* The sentinel is **carried end to end as the literal string
`none`** — it is NOT mapped to empty at any layer (RR-4P3WPD).

`v1.SidebarItem.Icon` is `json:"icon,omitempty"`, so an empty string is dropped
from the payload entirely and becomes indistinguishable from "field never set".
Worse, empty already means "use the derived icon" in YAML, so mapping `none` to
empty would give the same token opposite meanings on the two sides of the wire —
the exact overload the ticket rejects `icon: ""` to avoid.

One constant, `NoIcon = "none"` (Go) / `NO_ICON = 'none'` (TS), never a bare
literal.

- **Config layer**: `validateIconName` accepts `NoIcon` as a reserved name. It is
  not a table entry (it names no component), so it is an explicit case.
- **Wire layer** (`views_handler.go:385-387`): three-way — authored name wins;
  `none` passes through as `"none"`; absent keeps the derived icon.
- **SPA layer**: one shared helper, so the decision exists in exactly one place
  and the two surfaces cannot drift again (RR-D8I2R2):

  ```ts
  export function hasIcon(name?: string | null): boolean {
    return !!name && name !== NO_ICON
  }
  ```

  `isKnownIcon('none')` must stay **false** and `resolveIcon` must never be
  reached with it — guarded in the template, pinned by a test, so `none` can
  never be quietly added to the registry as a real glyph.

*4a. Collapsed sidebar* (RR-85NWY3). Collapsed mode hides `.nav-label`
(`Sidebar.vue:20-24`), so a `none` item would render as a blank-but-clickable
strip: invisible to a sighted user, a normal item to a screen reader. **When
collapsed, a `none` item falls back to its kind-derived glyph.** `none` means
"this row needs no glyph *to be told apart from its labelled siblings*";
collapsing removes the labels, so the premise is gone and the icon is the only
affordance left. The collapsed state therefore lives at the call site, not
inside `hasIcon`.

*5. Sidebar layout.* `.nav-icon` is `flex: 0 0 auto; margin-right: 18px` with
the 18px box coming from the `:size` prop (`Sidebar.vue` style block, lines
106-127 — with a comment warning that setting CSS `width` breaks Lucide's
presentation attributes and produced a 24×18 stretch, RR-5ESHXG). The spacer
must therefore reserve `18px + 18px margin` **without** touching `.nav-icon`'s
sizing rule: a sibling class with an explicit `width: 18px; height: 18px` on a
plain `<span>` (no SVG involved, so the presentation-attribute trap does not
apply), carrying the same `flex: 0 0 auto; margin-right: 18px`.

*6. Kanban.* The three render sites (`KanbanView.vue:574-580`, `:620-626`,
`:637-643`) switch from `v-if="column.icon"` to `v-if="hasIcon(column.icon)"`,
so `icon: none` renders nothing instead of falling through `resolveIcon` to the
default FileText glyph (RR-D8I2R2). No spacer: a column header has no alignment
column to reserve, so the *semantics* are shared while the *layout* legitimately
differs. This does not reopen RR-VESDBZ's unknown-name disagreement, which stays
wont-fix.

**Alternatives considered**

| Alternative | Rejected because |
| --- | --- |
| TS registry canonical, parse it to emit Go + docs | Perpetuates regex-parsing TypeScript, which `icons_test.go`'s own comment calls fragile. Go already needs the list for validation. |
| YAML canonical + embed | Adds a parse step and moves component-name typos from compile time to runtime, for no gain over a Go slice. |
| Keep two hand-written lists, generate only the docs table | Leaves the third copy unpinned and still costs three edits per icon; does not satisfy AC 1. |
| All ~1600 lucide names via dynamic resolution | Breaks the static-allowlist security property that `icons.ts`'s doc comment and `resolveIcon` exist to enforce, and defeats tree-shaking. |
| `icon: ""` for no-icon | Empty already means "use the derived icon"; reusing it silently changes the meaning of existing configs. |
| Group-level `icons: false` | Coarser than the HIG advice needs, and a per-item primitive can be composed into it later; the reverse is not true. |
| Collapse the icon column when absent | Ragged left edge in a mixed menu; HIG alignment expectation is the opposite. |

**Files to modify**

*New:*
- `internal/dataentryconfig/icondefs/icondefs.go` — canonical `[]IconDef` (~150), leaf package
- `cmd/gen-icons/main.go` — the generator (`-root` flag; `// coverage-ignore: generator entry point`)
- `internal/dataentryconfig/icons_gen.go` — generated `ValidIconNames`
- `frontend/src/utils/iconRegistry.generated.ts` — generated imports, `ICONS`, and named chrome exports
- `frontend/src/components/common/NavIcon.vue` — icon-or-spacer wrapper

*Changed:*
- `internal/dataentryconfig/config.go` — drop the literal `ValidIconNames`; `NoIcon` const; rewrite the unknown-icon message (RR-2VFFCD)
- `internal/dataentry/views_handler.go` — three-way icon resolution (~line 385)
- `frontend/src/utils/icons.ts` — keep `resolveIcon`/`isKnownIcon`/`DEFAULT_ICON`/`iconNames`; add `hasIcon` + `NO_ICON`; re-export the generated map
- `frontend/src/components/common/Sidebar.vue` — 4 nav render sites + 5 chrome refs → named imports (RR-Y1PO6R) + spacer CSS + collapsed fallback
- `frontend/src/views/KanbanView.vue` — 3 render sites → `hasIcon`
- `internal/dataentryconfig/icons_test.go` — replace the TS-parsing parity test
- `docs-project/entities/guides/GUIDE-data-entry.md` — generated table region; `icon: none` prose in the kanban (~2020) and navigation (~2262) sections; rewrite the "deliberately not repeated here" paragraph
- `docs/data-entry.md` — regenerated via `just docs`; never hand-edited
- `.go-arch-lint.yml` — component entries for `icondefs` and `cmd/gen-icons` (RR-8VBAJM)
- `.testcoverage.yml` — floor for the new package(s), if required
- `justfile` — `generate-icons` recipe, ordered **before** `docs`; extend the drift gate
- `.github/workflows/ci.yml` — run the drift check

**Dependencies:** none new. `lucide-vue-next` ^1.0.0 already present; Go stdlib
`text/template` for the generator.

*7. Unknown-icon message* (RR-2VFFCD). Joining ~150 names into every error is a
~1.6 KB wall per offending entry. The message becomes diagnostic rather than
exhaustive now that a real catalogue exists: nearest-match suggestion, an
explicit mention of `none` (not discoverable by similarity), and a pointer to
`docs/data-entry.md#icons` instead of the full list.

```
navigation "My Tickets": unknown icon "inbxo" (did you mean "inbox"?);
use "none" for no icon, or see docs/data-entry.md#icons for all 152 names
```

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation**

| Input | Source | Validation | On invalid |
| --- | --- | --- | --- |
| `icon:` on a nav entry | operator-authored `data-entry.yaml` | allowlist: generated `ValidIconNames`, plus reserved `none` | config error at load, listing every valid name |
| `icon:` on a kanban column/swimlane | same | same | same |
| `item.icon` on the wire | rela's own API, derived from the above | `resolveIcon`'s own-property check | falls back to `DEFAULT_ICON`, never throws |

The trust boundary is unchanged: icon names are operator config, not end-user
input, and per CLAUDE.md ("The configuration is not a secret") the set of valid
names is explicitly non-confidential — which is what makes publishing the full
table in end-user docs correct rather than a disclosure.

**Security-Sensitive Operations**

*The one real property here is that a config string must never name an arbitrary
Vue component.* It is preserved by construction:
- `ICONS` remains a statically-imported, closed map. The generator emits named
imports, so the bundler still tree-shakes and no dynamic `import()` or string →
component lookup is introduced.
- `resolveIcon` keeps its `Object.prototype.hasOwnProperty` check rather than a
bare index, so `toString` / `constructor` still return `DEFAULT_ICON` instead of
yielding a non-component and crashing the render. This logic stays
**hand-written**, not generated, precisely so it stays under review.
- Growing the map 16 → ~150 does not widen the surface: every value is still a
compile-time-known component reference.

`none` introduces no new resolution path — it is erased at the config/wire layer
and never reaches `resolveIcon`.

Generator file writes are build-time, developer-machine only, to fixed paths. No
user input reaches them. Error messages contain only config names, which are
non-secret by the rule cited above.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios** (one row per acceptance criterion)

| AC | Test | Where |
| --- | --- | --- |
| 1 Single source | Run the generator into a temp dir; assert the three outputs byte-match the committed ones | `icons_gen_test.go` (Go) |
| 2 Drift fails CI | Same test is the gate; failure message names `just generate-icons`. Plus `just ci` runs it | `icons_gen_test.go` + `justfile` |
| 3 Set size | Assert `len(ValidIconNames) >= 120`; assert every entry has a non-empty category and description; TS compile + a Vue test mounting **every** icon in `ICONS` and asserting an `<svg>` with `stroke="currentColor"` | Go + `icons.test.ts` (extends the existing loop at `icons.test.ts:22`) |
| 4 Docs table | Assert the guide entity contains the marked region with one row per name; **and** that `docs/data-entry.md` contains the same region, which is what proves the two generators chained in the right order | `icons_gen_test.go` |
| 5 `none` renders no glyph | Mount `Sidebar` with an item whose `icon` is empty; assert no `svg` in that row and that a spacer element is present | `Sidebar.iconRender.test.ts` |
| 6 `none` beats derivation | Table test over all seven entry kinds: `Icon: "none"` → wire `Icon == "none"` (**not** `""` — empty is also what a malformed entry yields, so asserting emptiness certifies nothing, RR-4P3WPD); `Icon: ""` → derived glyph; `Icon: "inbox"` → `"inbox"` | `views_handler` test (Go) |
| 7 `none` validates | `validateIconName("none", …)` → no error; `"nOnE"`, `"None"`, `"NONE"`, `""`, `"nope"` → expected outcomes. Message asserts the **suggestion** (`"inbxo"` → "did you mean inbox") and an explicit `none` mention — not "contains none", which a 150-name join satisfies vacuously (RR-2VFFCD) | `icons_test.go` |
| 8 No regression | Enumerate **all 16** current names (the existing test covers only 12) and assert **component identity**, not just presence. Separately assert the 6 chrome names resolve via the generated named exports, so a rename breaks the build rather than blanking a glyph (RR-Y1PO6R) | Go + `icons.test.ts` |
| 9 Security intact | Keep the existing `toString` / `constructor` / `undefined` / `null` / `''` fallback tests; add an assertion that `icons.ts` contains no dynamic import and that `ICONS` values are all functions/objects | `icons.test.ts` |
| — `none` never resolves | `isKnownIcon('none') === false`; `hasIcon('none') === false`; `hasIcon('')`/`hasIcon(null)` false; `hasIcon('inbox')` true (RR-4P3WPD, RR-D8I2R2) | `icons.test.ts` |
| — kanban `none` | A column and a swimlane with `icon: none` render no `svg`; one with `icon: inbox` does. Test Plan previously had **no** kanban row at all (RR-D8I2R2) | `KanbanView` test |
| — collapsed fallback | Collapsed sidebar + `none` item renders its derived glyph, so no row is both empty and interactive (RR-85NWY3) | `Sidebar.iconRender.test.ts` |
| — generator bootstrap | `icondefs` compiles independently of any generated file, so the generator runs even when `icons_gen.go` is broken (RR-8VBAJM) | build-level |

**Integration test approach.** AC 5 and 6 are the ones a unit test can pass
while the feature is still broken end to end, because they span three layers
(YAML → Go wire → SPA render). Covered by:
- a Go test loading a real `data-entry.yaml` fixture with `icon: none`, an
`icon: inbox` and a bare entry in one group, through config validation into the
sidebar wire payload, asserting all three shapes at once;
- a Vue test mounting `Sidebar` with that exact payload and asserting the mixed
group renders glyph / glyph / spacer with labels aligned.

Plus **manual verification** with `just dev` against a config exercising a mixed
group, since alignment is the acceptance criterion and no unit test really sees
it.

**Edge Cases**

| Case | Expected |
| --- | --- |
| `icon: none` on a group | Rejected — groups already reject any icon (`validate.go:476-481`); message must stay the group-specific one, not "unknown icon" |
| `icon: none` on a kanban column/swimlane | Accepted; renders no icon (already the no-icon path via `v-if`) |
| `icon: NONE` / `None` / `nOnE` | Rejected. Names are case-sensitive everywhere else; a case-insensitive exception here would be the only one |
| `icon: none` on an `action:` entry | Accepted; action entries derive no icon, so this is a no-op — must not error |
| A project literally wanting an icon named "none" | Impossible by design; called out in the docs |
| Empty `icon:` (`icon: ""`) | Unchanged: derived icon. Explicitly regression-tested, since this is the meaning `none` exists to avoid overloading |
| Duplicate name in the canonical table | Generator fails loudly rather than emitting a map with a silently-dropped duplicate |
| Two names → the same lucide component | Allowed (`document`/`FileText` is already a synonym of the default); must not trip the duplicate check |
| Sidebar collapsed + no-icon item | Collapsed mode hides `.nav-label`, so a no-icon item collapses to an empty row — verify it is not a dead click target |
| Icon name colliding with a JS reserved word / prototype key | `resolveIcon`'s own-property check already handles it; kept under test |

**Negative Tests**

- Unknown icon name → config load fails, message locates the entry (`columns[2]`)
and lists valid names including `none`. (Extends the existing assertions at
`icons_test.go` `TestValidateIconName`.)
- Hand-edited generated file → generator test fails naming the recipe.
- Canonical table entry naming a nonexistent lucide component → generated TS
fails to compile (`npm run build`), and the mount-every-icon test fails.
- Missing category or description on an entry → generator fails.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl) — **m**

**Risks**

| Risk | Likelihood | Impact | Mitigation |
| --- | --- | --- | --- |
| Bundle size grows with ~150 static icon imports | High (certain) | Low–medium | Named imports keep tree-shaking, but every icon in a static map is reachable, so all ~150 ship. At Lucide's ~0.3–0.5 KB/icon uncompressed this is roughly 40–70 KB raw / well under 20 KB gzipped. **Measure before/after and record the number in the implementation checklist**; if it exceeds expectation, trim the set rather than going dynamic (dynamic breaks AC 9). |
| Naming choices are a frozen contract | Medium | High | RR-OX9WFS already burned this ticket's predecessor: two names described a *use site* rather than the glyph and had to be renamed. Rule for the table: name after **what the glyph depicts** (`wrench`), not where it is used (`settings-page`). Reviewed as its own pass before merge. |
| Lucide alias removal at the next major | Medium | Medium | Already materialising — three current imports are deprecated aliases. Mitigated in this ticket by moving the table to canonical names (Approach §3). |
| Generated docs region conflicts with `just docs` | ~~Low~~ **Confirmed during planning** | Medium | **It is generated** — `docs/data-entry.md` line 1 says so, sourced from `docs-project/entities/guides/GUIDE-data-entry.md`. Resolved by writing the region into the guide entity instead, so the two generators chain rather than fight (see Approach §2). Ordering matters: `just generate-icons` must run **before** `just docs`; the drift gate must cover both files or a stale `docs/` copy passes. |
| Sidebar spacer regresses icon sizing | Low | Medium | RR-5ESHXG's 24×18 stretch came from CSS `width` beating Lucide's presentation attributes. The spacer is a plain `<span>`, never an SVG, and `.nav-icon`'s rule is untouched. Verified by the existing size assertions plus a new spacer-dimension test. |
| Replacing the parity test loses coverage | Low | Medium | The new generator test is strictly stronger (byte equality vs. regex-parsed name sets), but the *reason* for the old count check is preserved in the new test's comment so nobody reintroduces a weaker parse. |
| ~150 curated names is bikeshed-prone | Medium | Low | Categories and names proposed as one reviewable commit; the generator makes later additions cheap, so the initial set does not need to be perfect. |

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs-project/entities/guides/GUIDE-data-entry.md` (the **source**; `docs/data-entry.md`
is its generated output) — the generated icon table region; prose for `icon:
none` in **both** the "Column and swimlane icons" and "Item icons" sections; and
a rewrite of the paragraph at line 2020 that currently says the name list is
"deliberately not repeated here" — that rationale (RR-GTOQCF) is superseded once
the list is generated, and leaving it would read as a contradiction of the table
beside it. The replacement prose must say *why* it is safe now (generated,
drift-gated), so the next reader does not re-apply RR-GTOQCF and delete the
table.
- [x] `frontend/CLAUDE.md` / `icons.ts` doc comment — "Adding an icon" currently
instructs editing two lists by hand (`icons.ts:15-18`). Must become "edit the
canonical table and run the generator", or the next contributor follows stale
instructions into a failing drift check.
- [ ] ~~`docs/metamodel.md`~~ (N/A: icons are data-entry config, not metamodel)
- [ ] ~~`docs/cli-reference.md`~~ (N/A: no CLI surface; the generator is a `just` recipe)
- [ ] ~~`CLAUDE.md` (root)~~ (N/A: no new cross-cutting pattern — the generated-artefact
convention it follows already exists)
- [ ] ~~`README.md`~~ (N/A: no project-level change)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

| ID | Severity | Finding | Resolution in plan |
| --- | --- | --- | --- |
| RR-4P3WPD | critical | `none` encoded as empty string is erased by `omitempty` and re-creates the very overload the ticket rejects `icon: ""` to avoid; AC 6 was unassertable | Approach §4 — sentinel carried end to end as `"none"`; `NoIcon`/`NO_ICON` constant; AC 6 restated |
| RR-Y1PO6R | significant | 5 hardcoded `ICONS.<name>` template refs make 6 names a silent compile-time SPA dependency — `Record<string, Component>` doesn't error on a missing key, the glyph just vanishes | Generator emits named chrome exports; `Chrome bool` on `IconDef`; AC 8 asserts identity for all 16 |
| RR-8VBAJM | significant | Generator placement: unspecified root resolution across two invocation paths, uncovered arch-lint component, and a bootstrap deadlock (generator imports the package it writes into) | Table moved to leaf `icondefs`; generator to `cmd/gen-icons` with `-root`; single invocation path; arch-lint + coverage entries listed |
| RR-D8I2R2 | significant | `icon: none` on kanban would hit `resolveIcon` and render the default FileText glyph; Test Plan had no kanban row at all | Shared `hasIcon` helper across both surfaces; 3 KanbanView sites updated; kanban test row added |
| RR-2VFFCD | minor | Unknown-icon message joins ~150 names (~1.6 KB per bad entry) — a design sized for 16, kept after the constraint that justified it was removed | Approach §7 — suggestion + `none` mention + docs pointer; AC 7 restated |
| RR-85NWY3 | minor | Collapsed sidebar + `none` = a blank but focusable/clickable row: invisible to sighted users, normal to screen readers | Approach §4a — collapsed mode falls back to the derived glyph; test row added |

All six are addressed above; none deferred.
