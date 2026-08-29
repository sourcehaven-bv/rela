---
id: RR-B6QV5W
type: review-response
title: 'Assorted test-quality and docs findings: a vacuous Validate subtest, byte-wise edit distance, contradictory kanban prose, use-site icon names'
finding: |-
    Five smaller findings from the same review:

    **M5** — `TestValidate`'s "a valid table" case could not fail: the single-entry fixture always tripped the chrome check, so the assertion was "it failed for the reason I expected". Worse, it shared its exact input with the "chrome name missing" case — two subtests, identical data, opposite expectations, both passing.

    **M6** — `editDistance` compares bytes; nothing asserted icon names are ASCII. `strings.ToLower(name) == name` passes for `café`.

    **M7** — `Validate`'s doc comment defended a shared-Lucide-component case that no entry actually exhibits, and cited `document`/the fallback as an example when those are one entry referenced twice.

    **M8** — `-check` conflated "file missing" with "file stale": a permissions error or truncated read reported as "out of date; run just generate-icons", sending someone chasing a bug they do not have.

    **M9** — The kanban docs said omitting `icon:` and writing `icon: none` "do the same thing", while the sidebar section 250 lines later correctly said they differ. A future reader reconciling those would "simplify" `none` → `""` and silently un-fix RR-4P3WPD and RR-D8I2R2 in one commit.

    **L10** — Several NEW names described a use site rather than the glyph, violating the rule the table's own doc comment states. `deadline` → CalendarCheck was actively misleading (a checked calendar reads "done", not "due").
severity: minor
resolution: |-
    M5: added a `validTable()` fixture carrying the chrome names, so the valid case genuinely asserts `err == nil` and the switch collapses to a plain if/else. The chrome case now uses distinct input.

    M6: `Validate` rejects a non-ASCII name (`isASCIIKebab`), and `editDistance`'s doc states the byte-wise assumption and why it fails safe.

    M7: comment restated as forward-looking, noting no entry shares a component today.

    M8: `-check` distinguishes `os.IsNotExist` (stale, advice correct) from any other read error (returned as-is).

    M9: kanban prose now says the two are equivalent *for columns specifically*, because a column derives no glyph, and links to the sidebar section where they differ.

    L10: renamed the non-grandfathered use-site names — `deadline`→`calendar-check`, `identity`→`fingerprint`, `guide`→`book-open`, `priority`→`flame` (plus `contract`→`gavel`, `medical`→`stethoscope`, `lab`/`experiment`→`beaker`/`flask` from my own earlier pass). The four grandfathered ones (`dashboard`, `apps`, `document`, `warning`) were left alone — they ship and are pinned by the no-regression test.

    Also adopted the reviewer's best suggestion: the generated docs table now has a **Glyph column** naming the Lucide component. That self-disambiguates the confusable families (five Circle* glyphs differ only in their interiors at 18px) and makes a use-site name visible in review as a mismatched row.
status: addressed
---
