---
id: RR-NV753I
type: review-response
title: Allowlist drift test could silently stop covering a name (partial-parse false negative)
finding: |-
    TestIconAllowlistMatchesFrontend parsed icons.ts with a regexp guarded only by `len(spa) == 0`. That guard fires only when EVERY entry fails to parse — a global reformat. It cannot fire on a PARTIAL parse, which is what every realistic single-entry edit produces.

    The reviewer ran the exact regexp against realistic shapes and demonstrated five silent-drift cases: an aliased import (`Home as homeIcon`), a spread (`...navIcons`), a nested object literal, a commented-out entry, and a camelCase name. In each the new name is invisible to the parser, so if both sides are correct and consistent the test PASSES while having quietly stopped covering that name.

    The nested-literal case is worst: `strings.Index(body[start:], "\n}")` stops at the first column-0 brace, truncating the parse mid-map so everything after it is invisible forever.

    And the aliased-import case produces a FALSE POSITIVE with a misleading message — it blames the SPA for a name that renders fine.

    This is the failure mode I asked the reviewer to look for, and it was worse than I expected.
severity: significant
resolution: |-
    Fixed in bdb197f1 with two changes.

    The guard now asserts a COUNT against the Go list rather than non-emptiness. The two lists must be the same size by definition, so any parse regression — partial or total — becomes a failure. The message names the likely causes (spread, nested literal, aliased import) and prints what was parsed, so the next person isn't left guessing whether the lists differ or the parser broke.

    The regexp also matches key-only now (dropped the [A-Z] value anchor), so an aliased import no longer misattributes blame to the SPA side.

    Mutation-tested rather than assumed: adding a nested object literal fails with 'parsed 16 names but ValidIconNames has 15', and commenting out an entry fails with 'parsed 14'. Both were silent passes before.

    Not taken: the reviewer's suggestion to generate one side from the other (an icons.json or go:generate). It is the right end state — it deletes the parser entirely — but it is a build-pipeline change that does not belong in a PR about icons. Worth a follow-up ticket; the hardened test closes the actual hole in the meantime.
status: addressed
---
