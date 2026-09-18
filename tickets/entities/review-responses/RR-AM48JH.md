---
id: RR-AM48JH
type: review-response
title: ViewAddInfo is the forbidden carrier, and App is at its plimsoll method cap
finding: 'The plan named v1.ViewAddInfo as "the natural carrier" for the new affordance. Its doc comment (responses.go:1088-1092) explicitly forbids that: "Do not reach for this type from a new view-related response." RR-R8X6 on TKT-651W predicted this exact failure — that the misleading View prefix would tempt a future contributor to wire it into a view response "because the name suggested it was generic". Separately, App is pinned at //plimsoll:max-methods=89 / max-exported-methods=22 (app.go:192-193), grandfathered directives pinned to the current count precisely so they cannot grow, so adding the header-union assembly as an App method fails CI.'
severity: significant
resolution: 'Plan now defines new types (ViewSectionCreate / ViewSectionCreateTarget) rather than reusing ViewAddInfo, carried as omitempty fields on ViewSection and ViewResponse — omitempty keeps responses byte-identical for sections that did not opt in, which is what lets the guard test''s existing cases stay green unchanged. Header-union assembly is placed on viewsHandler (no plimsoll directive), not App. Arch-lint is unaffected: everything stays within already-coupled packages.'
status: addressed
---

## Finding

**Forbidden carrier.** The plan named `v1.ViewAddInfo` "the natural carrier".
Its doc comment (`responses.go:1088-1092`) says the opposite: "Do not reach for
this type from a new view-related response: the read-only-view invariant
established by TKT-651W means no view section should carry add affordances."

RR-R8X6 on TKT-651W predicted this precisely — the misleading `View` prefix
"lies about scope" and "a future contributor might wire 'ViewAddInfo' into a new
view-related response because the name suggested it was generic." That is what
happened, which is some evidence the rename (TKT-6ETQ) is still worth doing.

**plimsoll.** `App` is pinned at `//plimsoll:max-methods=89` /
`max-exported-methods=22` (`app.go:192-193`) — grandfathered directives pinned
to the current count so they cannot grow. Header-union assembly added as an
`App` method fails CI.

## Resolution

- New types `ViewSectionCreate` / `ViewSectionCreateTarget`, not `ViewAddInfo`.
- Carried as `omitempty` fields on `ViewSection` (per-section) and `ViewResponse`
(header union). `omitempty` keeps responses byte-identical for sections that did
not opt in — which is exactly what lets the guard test's five existing cases
stay green with no edit.
- `ViewResponse` is a flat 4-field struct, far under plimsoll's 20-field cap.
- Header-union assembly goes on `viewsHandler`, which carries no directive.
- Arch-lint unaffected: `internal/dataentry`, `internal/dataentryconfig` and
`internal/apiwire` are already coupled.
