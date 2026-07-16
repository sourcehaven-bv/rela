---
id: RR-8GRLD
type: review-response
title: '''Parse errors surface at author time'' has no mechanism — config is served, not compiled in the SPA'
finding: 'The plan repeatedly says malformed conditions ''surface at author time where possible''. But form config is authored in data-entry.yaml and served to the SPA as opaque JSON; Go explicitly does NOT parse the grammar (deferred), and the SPA only sees the string at render time. So there is no author-time surface today — a typo''d condition silently evaluates false in the browser, and the branch just never shows. That is a real footgun for a GDPR-register author. Required: decide the mechanism concretely. Options: (a) a tiny Go-side syntactic pre-check that reuses a shared grammar definition (rejected in plan to avoid a second parser — correct); (b) surface engine parse errors in the SPA (dev console + a visible config-error banner in the data-entry UI, which already renders config/validation problems); (c) a `rela` CLI lint that runs the engine grammar over the config. Pick at least one and specify it; ''where possible'' is not a plan.'
severity: significant
resolution: 'Mechanism decided (user): dev-console warn at runtime + a `rela` CLI config lint that reuses the Go internal/predicate parser (predicate.Compile with form/entity/current_user env) to flag parse errors and unknown refs — the shared-grammar payoff, no second Go parser. Documented caveat that predicate is stricter (flags a superset). In-SPA banner explicitly rejected. CLI lint''s Go wiring scoped to land with TKT-CHLAJ (when config first carries conditions); engine ticket owns the TS runtime + grammar spec.'
status: addressed
---
