---
id: IMPL-G0HGN7
type: implementation-checklist
title: 'Implementation: Replace CodeQL with Semgrep frontend DOM-XSS rules'
status: done
---

## Development

- [x] ~~Unit tests written for new code~~ (N/A: the change is workflow YAML and
Semgrep rule YAML; there is no test harness for either. Verified by executing
Semgrep against the repo and against a purpose-built canary, evidence below)
- [x] ~~Integration tests written~~ (N/A: the job is a GitHub Actions workflow;
the pinned container was run locally instead, same version, same arguments)
- [x] Happy path implemented (six rules, `--error` so a finding fails the job)
- [x] Edge cases handled (string literals excluded from every sink rule so
author-controlled markup does not fire; `--exclude` for `*.test.ts` / `*.spec.ts`)
- [x] Error handling in place (`--error` makes findings blocking rather than
advisory; the image tag is pinned to `1.140.0` so the ruleset cannot drift)

## Test Quality

- [x] ~~Fixture builders or factories~~ (N/A: no test suite)
- [x] ~~No hardcoded values in assertions~~ (N/A: no test suite)
- [x] ~~Only specifying values that matter~~ (N/A: no test suite)
- [x] ~~Interpolated values constructed from objects~~ (N/A: no test suite)
- [x] ~~Property comparisons use original object~~ (N/A: no test suite)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified
- [x] Edge cases manually verified

**Verification Evidence:**

Semgrep 1.140.0 (the exact version pinned in the job) run locally with the
job's own arguments:

```text
semgrep scan --config=.semgrep/frontend-xss.yml --error --metrics=off \
  --exclude='*.test.ts' --exclude='*.spec.ts' frontend/src e2e
```

Result: 6 rules run over 221 targets, ~100% of lines parsed, **0 findings,
exit 0**. The parse rate matters: a rule file that silently fails to parse its
targets would also report zero.

The rules are not inert. Run against a canary containing each sink, all six
rules fire — 9 findings:

- `dom-innerhtml-assignment`, `dom-outerhtml-assignment`,
  `dom-insert-adjacent-html` on a `location.search`-derived value
- `dom-document-write` on both `document.write` and `document.writeln`
- `js-eval-with-variable` on both `eval(x)` and `new Function(x)`
- `vue-v-html-raw-bypass` on `v-html="rawHtml"` and `v-html="unsafeBody"`

Negative cases correctly stay silent: the five string-literal forms
(`innerHTML = "..."`, `outerHTML = "..."`, `insertAdjacentHTML(pos, "...")`,
`eval("1+1")`, `textContent =`) and `v-html="renderedMarkdown"` produce no
findings. So the rules distinguish the sanitizer-bypass case from ordinary
correct usage rather than flagging every binding.

One real finding surfaced on the first run, in
`frontend/src/utils/markdown.ts`: `slot.innerHTML = DOMPurify.sanitize(...)`
inside `sanitizeMermaidSVG`. True positive by rule shape, safe in fact — the
assigned value is the sanitizer's own output. Annotated with
`nosemgrep: dom-innerhtml-assignment` plus the reason, as the rule message
prescribes. The rule was deliberately NOT weakened to exempt all
`DOMPurify.sanitize(...)` values, since that would also hide a misconfigured
sanitizer call.

Note for future edits: Semgrep honours `nosemgrep` only on the match line or
the line immediately above it. A comment block placed further up does not
suppress — confirmed by observing the finding persist, then clear once the
marker was moved adjacent to the match.

## Quality

- [x] Code follows project patterns (job pinned by digest-style version tag like
the other tool jobs; comments explain the why, not the what)
- [x] Checked for DRY opportunities — the rules deliberately do not duplicate
gosec's coverage; `.golangci.yml` now names the split so the two halves are
discoverable from either side
- [x] No security issues introduced — this is a net change in scanner coverage;
see the review checklist for the CodeQL-versus-Semgrep coverage analysis
- [x] No silent failures — `--error` is what prevents the job from being a green
checkmark over an unread ruleset; `--metrics=off` keeps the scan offline
- [x] No debug code left behind
