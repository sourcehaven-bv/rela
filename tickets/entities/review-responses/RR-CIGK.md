---
id: RR-CIGK
type: review-response
title: evalCall evaluates args before checking host fn exists
finding: 'eval.go:113-131: args are evaluated in a loop, then s.bindings.Funcs[n.name] is looked up. If the host fn is missing, the work done evaluating args is wasted; more importantly, if an arg subtree itself fails (e.g. another missing host fn), the error message shows the inner failure while the outer call was always going to fail. Move the s.bindings.Funcs lookup to the top of evalCall, before the arg loop.'
severity: significant
resolution: Moved the host-fn lookup to the top of evalCall, before the arg evaluation loop. A missing host fn now fails fast without spending the step budget on subtree evaluation; a misleading inner error from an arg subtree no longer masks the outer missing-fn problem.
status: addressed
---
