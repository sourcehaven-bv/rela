---
id: RR-9CQTQO
type: review-response
title: Two of three .prose sites were defended only by the golden snapshot
finding: 'The .prose class introduced to scope markdown-table styling is applied by hand at three sites (intro, section body, footer). The reviewer confirmed all three are correct, but mutation-tested the GUARD: removing .prose from the section body fails TestRender_StylesMarkdownTables, while removing it from intro or footer fails nothing except TestRender_Golden. A golden snapshot is the artifact most likely to be regenerated on autopilot when it goes red, and its failure message says "output changed", not "you just unstyled every table in the footer". Since .prose exists precisely to stop a styling bug that only a human eye caught, leaving two of its three sites to a snapshot was the wrong defense.'
severity: significant
resolution: 'TestRender_StylesMarkdownTables is now table-driven over all three markdown-landing sites, each as its own named subtest. Verified by mutation: removing .prose from intro or from footer now fails that site''s own subtest by name, rather than only shifting a snapshot. The template restored byte-identical afterwards. Also fixed a related pre-existing defect the reviewer surfaced (RR minor #3): .pad p never reached the footer, since .foot is a sibling cell rather than a descendant, so multi-paragraph footers rendered jammed together. A single .prose p rule now spaces all three sites uniformly — pinned by TestRender_FooterParagraphsAreSpaced.'
status: addressed
---

## Finding

Reported by cranky-code-reviewer, who answered the question asked ("did any
markdown site miss the class?") and then went further to test the guard itself.

**The scoping was correct** — intro, section body and footer all carry `.prose`,
confirmed by rendering GFM tables through each. The problem was the defense.
Removing the class from each site in turn and running everything *except*
`TestRender_Golden`:

| Site | Non-golden test that catches it |
|---|---|
| section body | `TestRender_StylesMarkdownTables` |
| intro | **none** |
| footer | **none** |

That is a weak place to rely on a snapshot. A golden file is the artifact a
developer regenerates on autopilot when it goes red, and its message says
"output changed", not "you just unstyled every table in the footer". The whole
reason `.prose` exists is a styling bug that only a human eye caught — so
leaving two of its three sites to the snapshot repeats the original mistake.

## Resolution

`TestRender_StylesMarkdownTables` is now table-driven over all three sites, each
a named subtest rendering a GFM table and asserting no bare `<th>`/`<td>`
survives.

**Verified by mutation**, not by assumption: removing `.prose` from the intro
fails the `intro` subtest; removing it from the footer fails the `footer`
subtest. The template was restored byte-identical afterwards.

### Also fixed: the footer paragraph margin (reviewer's minor #3)

While in this code, the reviewer noted `.pad p { margin:0 0 12px 0 }` never
reached the footer, because `.foot` is a sibling cell rather than a descendant
of `.pad`. A multi-paragraph footer rendered with its paragraphs jammed
together. Confirmed in output: intro paragraphs carried a margin, footer
paragraphs were bare `<p>`.

Pre-existing on `develop` rather than introduced here, but `.prose` is exactly
the right fix and this was the moment: one `.prose p` rule now covers all three
markdown sites uniformly, where `.pad` covered two and silently missed the
third. Pinned by `TestRender_FooterParagraphsAreSpaced`, which asserts no bare
`<p>` survives anywhere in the document.
