---
id: RR-L9GJBS
type: review-response
title: 'Security review: no findings across the mail render path'
finding: Security review of the mail render path (pipeline order, lang attribute injection, dark-mode CSS validation, sanitizer bypass, safeHref/BaseURLCarrier, header injection, arch-lint trust boundary) returned no findings. Every claim verified by execution rather than reading.
severity: minor
resolution: 'No code changes required. Recorded for traceability: the diff adds a new untrusted-input surface (mail.render) to a security-relevant path, so evidence that it was reviewed and cleared is worth keeping alongside the ticket.'
status: addressed
---

## Finding

Security review of the full diff (rela-security-reviewer), framed by the OWASP
web application checklist and rela's own mail invariants.

**Result: no security findings.** Recorded as a review-response rather than left
implicit, because "a reviewer looked and found nothing" is evidence worth
keeping — particularly for a diff that adds a new untrusted-input surface to the
mail path.

Surfaces reviewed: `internal/mailrender` (pipeline, palette/CSS, the new `lang`
attribute), `internal/lua` (the new binding), `internal/mail` (the config
capability), `internal/mailtemplate` (load validation), `.go-arch-lint.yml`.

## Resolution

Each claim was verified by execution, not by reading:

- **Pipeline order intact.** `Render` still ends at `inliner.Inline`, and the
Lua path constructs a `mailrender.Renderer` rather than reimplementing a step,
so it inherits the order. `newMailPolicy` still withholds `style`, so untrusted
content cannot contribute CSS for douceur to inline.
- **`lang` cannot escape the attribute.** `langRe` is anchored at both ends and
admits no quote, angle bracket, whitespace, CR, LF or NUL, with a 35-byte cap.
`en" onload="alert(1)` was confirmed rejected on all three paths — `New`,
`Render`, and the Lua binding (which raises rather than defaulting).
- **The placement of that validation was confirmed correct**: in `resolveLang`
inside `buildDocument`, because the value arrives from operator config AND
untrusted Lua, and validating at either call site would leave the other open.
- **Dark-mode CSS is fully validated.** `ValidatePalette` iterates the whole
merged map and is key-agnostic, so the seven new `--dark-*` tokens were covered
the moment they existed — there was no allowlist to forget to extend. An
override of `--dark-bg-color` with `url('javascript:alert(1)')` is rejected at
`New`.
- **No sanitizer bypass.** Probed `</h1><script>`, `<img onerror=>`,
`<b onclick=>` and `[x](javascript:...)` through subject, titles, columns,
cells, intro, body and footer. Every one arrived escaped or stripped.
- **`BaseURLCarrier` does not widen anything.** It is a pure getter over
operator config; a script cannot set it, mirroring how `from` and the recipient
policy are withheld. (A script could always put an absolute
`https://evil.example` link in a row — that is a pre-existing property of any
script permitted to compose mail bodies, not something this interface adds.)
- **Header injection unaffected.** `mail.render` touches no transport; the
CR/LF/NUL gate stays at the `mail.send` enqueue boundary, unmodified.
- **No cycle.** `go list -deps ./internal/mailrender` returns exactly one
internal package — itself. `lua → mail` remains forbidden.

**One non-finding worth keeping:** `baseURLFor` type-asserts on `r.mailSender`,
which is legitimately nil when mail is unconfigured. A type assertion on a nil
interface yields `ok == false` rather than panicking, so this returns `""` and
`safeHref` drops relative links — the fail-closed direction. Verified by test
(`TestMailRender_WorksWithoutMailConfigured`) and consistent with the CLAUDE.md
rule about nil collaborators: the sender is genuinely optional here, since
rendering needs no transport.

**Explicitly out of the reviewer's scope:** whether the `@media` block renders
correctly in the clients it targets. That is a compatibility question, not a
security one, and it is what the vendored dataset check and the manual
two-scheme visual verification cover.
