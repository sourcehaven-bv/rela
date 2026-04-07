---
id: RR-NM5I
type: review-response
title: Vue dev server (Vite) on :5173 will be blocked by Origin allowlist
finding: 'vite.config.ts proxies :5173 → :8080 with changeOrigin:true, which rewrites Host but leaves Origin as `http://localhost:5173`. The plan derives the Origin allowlist from the bind address only, so dev mode breaks: every fetch from the SPA in dev would be 403''d. The plan does not mention this at all.'
severity: significant
resolution: 'Plan updated: `--bind` accepts an additional `--allowed-origin` flag (repeatable) so devs can pass `--allowed-origin http://localhost:5173`. Default config in `vite.config.ts` already targets the dev port; document the dev startup command in `docs/security.md`. Alternative considered: auto-allow loopback dev ports — rejected because it widens the allowlist permanently and the explicit flag makes the dev exception visible.'
status: addressed
---
