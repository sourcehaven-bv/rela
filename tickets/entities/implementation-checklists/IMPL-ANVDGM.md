---
id: IMPL-ANVDGM
type: implementation-checklist
title: 'Implementation: Extend the icon set to a curated ~150 names, generate registry + docs from one source, add icon: none'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

New tests, by layer:

| File | Covers |
| --- | --- |
| `cmd/gen-icons/main_test.go` | The drift gate (AC 1, 2), table guardrails, the `Component` alias collision, chrome exports, docs completeness |
| `internal/dataentryconfig/icons_test.go` | `none` validation + case-sensitivity, message length, deterministic suggestions, the 16-name no-regression pin |
| `internal/dataentry/sidebar_icon_test.go` | Three-way wire resolution across all 8 entry kinds (AC 6) |
| `internal/dataentry/icon_e2e_test.go` | YAML → validate → wire → **JSON**, the full path (AC 6) |
| `frontend/src/utils/icons.test.ts` | `hasIcon`/`NO_ICON` semantics, allowlist security property (AC 9), 16-name pin (AC 8) |
| `frontend/src/components/common/NavIcon.test.ts` | Spacer, alignment, collapsed fallback (AC 5) |
| `frontend/src/components/common/Sidebar.iconRender.test.ts` | Kanban `none` gating (the RR-D8I2R2 gap) |

Errors are surfaced, not swallowed: the generator returns on every failure and
`main` exits non-zero; the drift message names `just generate-icons`; an unknown
icon is still a load-time config error.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Table-driven with `t.Run` subtests throughout. Assertions reference
`dataentryconfig.NoIcon` / `NO_ICON` rather than the literal `"none"`, so the
sentinel has one definition in tests as well as in code.

Two places use literals **deliberately**, and say so in a comment: the 16
pre-existing icon names (deriving them from the thing they pin would defeat the
pin) and the per-kind derived glyphs in the wire test (the switch under test is
what maps them).

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

*AC 1 — single source.* `internal/dataentryconfig/icondefs` is the only
hand-edited definition. Renaming `tree` → `hierarchy` (a real duplicate the
validator caught during implementation) took one edit plus `just
generate-icons`; all three artefacts updated with no other change.

*AC 2 — drift fails.* Appended a line to `icons_gen.go` and ran the gate:

```
--- FAIL: TestGeneratedFilesAreUpToDate
    these generated files are out of date:
      internal/dataentryconfig/icons_gen.go
    ...  Run:      just generate-icons
```

Restored, re-ran, passed.

*AC 3 — set size.* `gen-icons: wrote 217 icons to 3 files`. All 217 Lucide
identifiers verified to resolve against the installed `lucide-vue-next@1.0.0`
(`imports: 217 missing: 0`); `NavIcon.test.ts` asserts inline `<svg>` with
`stroke="currentColor"` at 18×18.

*AC 4 — docs table.* `just docs` propagated the region into `docs/data-entry.md`
line 2309, categorised, one row per name, reachable from both the kanban and
navigation icon sections.

*AC 5, 6 — `none`.* `TestIconNoneEndToEnd` prints the actual wire payload for a
mixed group:

```
My Tickets     → {"label":"My Tickets",...,"icon":"inbox"}
Open Tickets   → {"label":"Open Tickets",...,"icon":"none","derivedIcon":"list"}
All Tickets    → {"label":"All Tickets",...,"icon":"list"}
```

All three intents distinct on the wire — which is the property RR-4P3WPD showed
an empty-string encoding would have destroyed.

*AC 7 — validation.* `icon: none` accepted; `None`/`NONE`/`nOnE` rejected;
`inbx` suggests `inbox`; message is 3 lines, not a 200-name wall.

*AC 8 — no regression.* All 16 original names pinned in Go and TS. Three were
deprecated Lucide aliases (`Home`, `AlertTriangle`, `CheckCircle2`) and now use
the canonical components (`House`, `TriangleAlert`, `CircleCheck`) — same
glyphs, verified by component identity, config names unchanged.

*AC 9 — security.* `ICONS` is still a closed, statically-imported map; the
`hasOwnProperty` check and its prototype-key tests are untouched; a test asserts
every value is a real component. No dynamic import introduced.

*Naming pass (RR-OX9WFS).* Audited all 216 names against "name the glyph, not
the use site" and renamed four of my own before merge: `contract` → `gavel` (a
gavel depicts a courtroom, not a contract — an author picking it for a contracts
list would get a judicial hammer), `medical` → `stethoscope` (named a domain,
not the glyph), and the confusing near-pair `lab`/`experiment` → `beaker`/`flask`
(`lab` named a place, and two similar vessels under unrelated names invited
picking the wrong one). The seven Navigation names that look use-site-shaped
(`dashboard`, `list`, `kanban`, `search`, `settings`, `apps`, `document`) are
pre-existing frozen contracts and were deliberately left alone. Status names
(`done`, `blocked`, `pending`) are semantic, not use-site: they stay correct on
every surface, which is the property the rule is actually protecting.

*Edge cases.* Group + `none` → group-specific message (not "unknown icon");
action + `none` → accepted no-op; `icon: ""` → still derives; duplicate table
name → generator fails; two names sharing a component → allowed; collapsed +
`none` → derived glyph returns.

**Bundle size** (the risk the plan committed to measuring): built both sides.
Baseline 6,533,701 B → 6,584,748 B, **+51,047 B (+0.8 %)** raw across all assets
for 200 additional icons. Within the 40–70 KB estimate; no trimming needed.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

*Patterns.* Generator output mirrors the existing generated-artefact convention;
the suggestion helper follows the `suggestForm`/`suggestView` precedent (but
scans sorted, because those iterate a map and are non-deterministic on ties);
component/test layout matches the package.

*DRY.* Three extractions, each because it sharpened a contract rather than for
its own sake: `NoIcon`/`NO_ICON` (the sentinel appeared in five places),
`hasIcon` (the sidebar and kanban must agree on when there is no glyph — the
RR-D8I2R2 fix), and `NavIcon.vue` (four near-identical sidebar render sites;
four copies of a conditional is how these surfaces drifted last time).
`lucideAlias` and `chromeExport` are one-liners kept as named functions so the
generated identifiers have one definition.

*Security.* The static-allowlist property is preserved by construction — see AC
9. `none` never reaches `resolveIcon` (guarded, and pinned by a test).

**Post-review revisions.** The cranky-code-reviewer found four must-fix issues
(RR-1YIOG5 critical, RR-8D1X16 / RR-1AKC31 / RR-82519A significant, RR-B6QV5W
minor) — two of which were design-review findings this checklist had claimed
addressed but had only half-fixed. All verified before acting, all reproduced,
all fixed. See REV-RG7YHP for the table. The evidence above was re-verified
against the revised code.

**Gates run:** `go test ./...` (exit 0) · `npx vitest run` (2020 passed) · `just
lint` (exit 0) · `npm run lint` (0 errors) · `just arch-lint` (OK — needed new
`icondefs`/`cmdGenIcons` components, exactly as RR-8VBAJM predicted) · `just
comment-lint` (11122 comments, no unresolvable links) · `just plimsoll` · `just
lint-md` (254 files, 0 issues) · `vue-tsc -b` · `npm run build`.
