---
id: RR-0PEI9F
type: review-response
title: 'link: security prose misplaces the protection; startsWith(''/'') admits //evil.com'
finding: 'The plan claimed column link: values become a new javascript: XSS sink. Verified false: resolveLinkTarget is a closed allowlist over ''detail'' and ''document/*'' in BOTH the Go copy (internal/dataentry/views_handler.go:713-725) and the TS mirror (EntityList.vue:475-483); everything else returns empty. Separately, the stated guard startsWith(''/'') admits protocol-relative //evil.com.'
severity: significant
resolution: 'Security prose corrected — the plan now records this as integrity/defence-in-depth, explicitly NOT confidentiality, per CLAUDE.md''s ''write down which of the two you mean''. Predicate tightened to /^\/(?![/\\])/ (also rejecting the backslash variant, RR-5Z83S0) and wired into EntityList.entityTarget, the one path where a server-supplied cellLink reaches an href. The allowlist itself is now pinned on both sides: a new Go test (internal/dataentry/link_target_test.go) asserts 13 hostile inputs resolve to empty, and TS tests assert no hostile link: value is ever bound as an href.'
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
