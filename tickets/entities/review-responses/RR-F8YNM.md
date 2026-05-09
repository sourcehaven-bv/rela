---
id: RR-F8YNM
type: review-response
title: Inaccessible.Path/Reason fields under-specified; consider info-leakage
finding: 'Plan does not pin down: (1) Is Inaccessible.Path project-relative (entities/feature/FEAT-001.md) or absolute (/Users/.../FEAT-001.md)? Pin to project-relative. (2) Reason is a free-form string; should be a typed enum (InaccessibleReason constant) so SPA branches on a known value, not a localized string. (3) For a future hosted/multi-tenant rela-server deployment, returning the on-disk path in HTTP response is info disclosure. Add explicit non-goal: ''data-entry runs locally; this surface is not safe for multi-tenant deployments without redaction.'' For relations: Inaccessible.Path is what — relations/FROM--type--TO.md? Spell it out.'
severity: significant
resolution: 'Path/Reason concerns mostly moot now that there''s no separate Inaccessible value type. Reason is a typed enum (InaccessibleReason) with constant InaccessibleReasonGitCrypt. No filesystem path leaks through API — the entity itself carries only ID/Type/Inaccessible, which the SPA was always going to receive. Info-leakage concern noted in plan: SPA shows ''git-crypt encrypted'' text, no path. Multi-tenant deployment caveat noted as out of scope.'
status: addressed
---
