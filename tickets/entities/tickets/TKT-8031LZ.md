---
id: TKT-8031LZ
type: ticket
title: Replace CodeQL with hand-written Semgrep frontend DOM-XSS rules
kind: chore
priority: medium
effort: s
status: done
---

Swap the CodeQL workflow for a Semgrep job carrying local, hand-written
DOM-XSS rules for the frontend.

## Why replace rather than add

CodeQL's value here was the `javascript-typescript` analysis over `frontend/`.
The Go half is already covered by gosec (see `.golangci.yml`, where the G702-G706
taint checks are now enabled), so running both scanners meant paying for two
SAST pipelines to cover one language.

Semgrep is faster, its rules are readable YAML that lives in the repo, and a
finding points at a rule the team wrote and can change. CodeQL's
`security-and-quality` pack is a black box by comparison, and its results land
in the Security tab rather than failing the job.

## Why the rules are hand-written

Semgrep's OSS registry packs contain no browser DOM-XSS rules. Verified by
canary: a literal `location.search -> innerHTML` flow produces zero findings
under `p/xss`, `p/javascript`, `p/typescript` and `p/owasp-top-ten`. Pointing
the job at a registry pack would have produced a permanently green check that
never inspected the risk it was named after.

`.semgrep/frontend-xss.yml` therefore defines six rules covering the HTML sinks
that matter in this codebase: `innerHTML`, `outerHTML`, `insertAdjacentHTML`,
`document.write`/`writeln`, `eval`/`new Function`, and `v-html` bound to an
explicitly raw value.

## Deliberate non-goals

No blanket `v-html` rule. `v-html` is used throughout the SPA to render markdown
already sanitized by `renderMarkdown()` (DOMPurify), so flagging every binding
produces noise on correct code and trains reviewers to ignore the rule. Only the
explicit sanitizer bypass is flagged.

## Branch protection

`CodeQL` is not a required status check on `develop` (the ruleset requires Test,
Lint, Fuzz, Docs, Rela Tickets, E2E, Architecture, God-object lint, Postgres
Backend, Frontend, Build, Demos and the six Cross-Compile jobs), so deleting the
workflow does not strand the merge queue. The new `Semgrep` job is likewise not
required yet; adding it to the ruleset is a follow-up once it has run green on
develop.

## Changes

- `.github/workflows/codeql.yml` — deleted
- `.semgrep/frontend-xss.yml` (new) — six DOM-XSS rules
- `.github/workflows/ci.yml` — adds the `Semgrep` job, pinned to
  `semgrep/semgrep:1.140.0`, running with `--error` so a finding fails the build
- `.golangci.yml` — notes that gosec is the Go half of the same split
- `frontend/src/utils/markdown.ts` — `nosemgrep` annotation on the one
  sanitizer-output `innerHTML` site, with the reason
