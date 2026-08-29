---
id: RR-0PEI9F
type: review-response
title: 'link: security prose misplaces the protection; startsWith(''/'') admits //evil.com'
severity: significant
status: addressed
---

**Finding (S1, significant).** The plan's security prose misidentifies where the
protection lives, which is how the real allowlist gets relaxed later.

Verified: `resolveLinkTarget` is a closed allowlist over two literal shapes in
**both** copies — Go (`internal/dataentry/views_handler.go:713-725`) and TS
(`EntityList.vue:475-483`). Input space is `{"", "detail", "document/*"}`;
everything else returns `""`. `javascript:`, `data:`, `//evil.com`,
`JavaScript:` all hit the default branch. An operator cannot express a
scheme-bearing link today, so there is **no new XSS sink** — the plan's original
claim that this was "the one genuinely new security check" was wrong and has
been corrected.

Two consequences:

- Keep a check, but framed as a **render-layer defence-in-depth invariant**, never as mitigation for a live threat. A future reader who believes the href check is load-bearing may conclude `resolveLinkTarget`'s allowlist is safe to loosen.
- The stated rule `startsWith('/')` is **incomplete**: it admits protocol-relative `//evil.com`. Correct predicate is `/^\/(?!\/)/`.

What actually protects this is a unit test on `resolveLinkTarget` (both copies)
pinning that unknown input returns `""` — that fails if someone adds a
passthrough branch.

Per CLAUDE.md's "write down which of the two you mean": this is **integrity /
invariant preservation, explicitly not confidentiality**. "Config is not secret"
is irrelevant here; the relevant fact is that editing `data-entry.yaml` already
requires repo write access.
