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

## Why delete the workflow

`codeql.yml` is redundant. CodeQL **default setup** is configured at the
repository level (actions, go, javascript-typescript, python; enabled
2026-09-04), so the workflow was a second, advanced-setup CodeQL run layered
over it. TKT-QM2VQ added the file when default setup was `not-configured`;
that is no longer the case.

Deleting the file does not remove CodeQL analysis — verified on the PR, where
the `Analyze (go)` / `Analyze (javascript-typescript)` / `Analyze (actions)` /
`Analyze (python)` checks all ran and passed on the commit that deletes it.

## Why add Semgrep

CodeQL's JS analysis was not catching browser DOM-XSS in this codebase, which
is the risk that matters most in `frontend/` — it renders user-authored
markdown to HTML. Semgrep rules are readable YAML in the repo, and they run
with `--error` so a finding fails the build rather than landing in the
Security tab. The Go side stays covered by gosec's G702-G706 taint checks,
now enabled in `.golangci.yml`.

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
Lint, Lint Markdown, Fuzz, Docs, Rela Tickets, E2E, Architecture, God-object
lint, Postgres Backend, Frontend, Build, Demos and the six Cross-Compile jobs),
so deleting the workflow does not strand the merge queue. The new `Semgrep` job
is likewise not required yet; adding it to the ruleset is a follow-up once it
has run green on develop.

## Changes

- `.github/workflows/codeql.yml` — deleted
- `.semgrep/frontend-xss.yml` (new) — six DOM-XSS rules
- `.github/workflows/ci.yml` — adds the `Semgrep` job, pinned to
  `semgrep/semgrep:1.140.0`, running with `--error` so a finding fails the build
- `.golangci.yml` — notes that gosec is the Go half of the same split
- `frontend/src/utils/markdown.ts` — `nosemgrep` annotation on the one
  sanitizer-output `innerHTML` site, with the reason
