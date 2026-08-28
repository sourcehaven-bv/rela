---
id: RR-61UFRU
type: review-response
title: Dead Services.mailOutbox field; SVG rule documented but never enforced; empty password authenticated blindly
finding: 'Three separate gaps. (1) Services.mailOutbox was written and never read — no accessor, no consumer — so the outbox looked wired when nothing could enqueue into it. (2) The Options doc pointed at a package-doc rationale for refusing SVG logos that did not exist, and no code path enforced raster-only; InlineImage.ContentType was a free string passed straight to the mail library. (3) With Username set and the named password env var unset or empty, the client authenticated with an empty password: the server rejects it, the outbox retries five times with backoff, burning ~30s and risking a relay lockout for a one-line config mistake.'
severity: significant
resolution: (1) Field removed — the mail import in appbuild disappeared with it, confirming it was the only reference; only mailStop is retained because Close genuinely uses it, with a comment saying the handle returns when TKT-U2R7GU needs it. (2) The rationale is now written in the package doc, and Message.Validate enforces a raster ALLOWLIST (png/jpeg/gif/webp) rather than an SVG denylist — the safe set is small and stable, the active-content set is not. (3) Send fails fast naming the empty variable, and Config.Validate requires password_env whenever username is set. All three pinned by tests.
status: addressed
---
