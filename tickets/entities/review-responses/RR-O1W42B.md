---
id: RR-O1W42B
type: review-response
title: 'rela-desktop: refuseRunAsEnv is inserted under main''s coverage directives'
finding: "At cmd/rela-desktop/main.go:1063-1075, the new func sits right after `// coverage-ignore-func: main function` and the `coverage-ignore-start` block comment. Both directives and the \"process start\" prose are now part of refuseRunAsEnv's doc comment, and main() no longer has its coverage-ignore-func line.\tGodoc for refuseRunAsEnv starts with coverage directives. A coverage tool that attaches coverage-ignore-func to the next declaration now exempts the wrong function.\tMove refuseRunAsEnv (with its own doc comment) above the `// coverage-ignore-func` line, so the directives stay attached to main."
severity: minor
resolution: refuseRunAsEnv moved above main's coverage directives with its own doc comment and coverage-ignore-func line; main's directives are back as before.
reason: refuseRunAsEnv moved above main's coverage directives with its own doc comment and coverage-ignore-func line; main's directives are back as before.
status: addressed
---
