---
id: RR-S0I1WH
type: review-response
title: douceur performs zero CSS value validation — it runs after the sanitizer and can inject javascript:/behavior:/expression() into style attrs
finding: 'The pipeline places douceur/inliner LAST, after bluemonday. Verified against douceur v0.2.0: inliner.Inline performs no CSS value validation whatsoever, materializing background:url(''javascript:alert(1)''), behavior:url(x.htc) (IE HTC script execution) and width:expression(alert(1)) directly into style attributes AFTER the only sanitizer has run. The last transform in the pipeline is therefore an unvalidated attribute injector. Today the <style> block is operator-authored so this is not directly attacker-controlled, but the plan bakes the ordering in as a structural invariant, and the palette map is already such a channel: PLAN passes map[string]string colour tokens into CSS with no validation that a colour is a colour. Verified that an accent token of url(''javascript:alert(1)'') lands verbatim in a style attribute.'
severity: critical
resolution: Two defences added to the plan. (1) Palette tokens are validated at the mailrender boundary with an ALLOWLIST (^#[0-9a-fA-F]{3,8}$ or a named-colour set) and REJECTED, not escaped, so no palette value can reach CSS unvalidated. (2) The invariant is recorded that only the trusted template's <style> block feeds the inliner — untrusted content is sanitized to a fragment before templating and never contributes CSS. Added an acceptance criterion asserting post-inline that no javascript:/behavior:/expression( value can appear in any style attribute; the existing sanitize AC could not detect this.
status: addressed
---
