---
id: RR-NK0VW9
type: review-response
title: 'npm run dev gets neither injection nor @layer: dev and prod have different cascades'
finding: 'The spike plugin uses generateBundle, a build-only Rollup hook that never runs under the Vite
  dev server. Separately the Go spaHandler injection never applies in dev either, because npm run dev
  serves frontend/index.html from Vite rather than the embedded shell, and in dev Vite injects CSS as
  JS-driven <style> tags rather than <link>s. So under npm run dev there is no @layer and no injection
  — a completely different cascade from production. A frontend developer verifying a styling change locally
  is testing something other than what ships. The plan''s file list touches vite.config.ts but never mentions
  the dev server. Note the e2e path is saved by luck rather than design: npm run build:e2e (justfile:391,
  ci.yml:269) is a real ''vite build --mode development'', so generateBundle DOES run and e2e sees the
  layer — worth stating explicitly so nobody later ''optimizes'' e2e onto the dev server and silently
  loses the coverage.'
severity: significant
status: addressed
resolution: 'Accepted and documented in frontend/CLAUDE.md: the wrap is build-only, so npm run dev has
  no layer and cascade-sensitive changes must be verified against npm run build. Also recorded that build:e2e
  IS a real vite build so e2e exercises the layer, and that nobody should move e2e onto the dev server.'
---

## Recommended resolution

Decide and document whether dev-mode divergence is accepted.

If yes (defensible), say so in `frontend/CLAUDE.md` — already a planned doc
target: *"the `@layer` wrap is build-only; verify cascade changes against `npm
run build`, not `npm run dev`."*

If no, add a `transform`-hook arm so the layer also applies in dev.

Accepting the divergence is fine; leaving it undiscovered is not.
