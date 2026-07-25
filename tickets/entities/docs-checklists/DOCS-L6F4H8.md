---
id: DOCS-L6F4H8
type: docs-checklist
title: 'Docs: fail-closed data-entry script read wiring'
status: done
---

## Code docs

- [x] `App.scriptReader` godoc rewritten — states the real justification (the unattended IdP webhook consumer), names all three consumers, and documents the deliberate divergence from appbuild's nil-redactor guard with an explicit warning against "aligning" them.
- [x] `App.scriptTracer` godoc cross-references the reader's rationale.
- [x] `appbuild.luaReadDepsFor` godoc corrected — it described the pre-RR-GKCZO5 fail-open behavior while the code 30 lines below failed closed (RR-DZ7H2S).
- [x] `visibility.DenyTracer` godoc corrected — it cited `FindOrphans` as the diagnostic surface, but no script can reach it; the caveat is now stated honestly (RR-R38TA9).

## Project docs

- [x] `docs-project/entities/guides/GUIDE-acl-security.md` — added a bullet to "ACL fail-loud and middleware scope" covering what happens when the read gate cannot be built, including the traversal asymmetry (refused traces look empty to a script) and the explicit note that policy-less projects are unaffected.
- [x] Regenerated derived docs via `just docs` (`docs/acl-security.md`); source-of-truth edited, not the generated copy.

## External docs

- [x] ~~API/OpenAPI docs~~ (N/A: no HTTP surface change — this is internal wiring)
- [x] ~~Migration notes~~ (N/A: no operator action required; the change only alters behavior on a fault that is currently unreachable)
- [x] Issue #1198 to be answered with both premise corrections stated explicitly.
