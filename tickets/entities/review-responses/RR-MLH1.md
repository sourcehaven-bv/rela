---
id: RR-MLH1
type: review-response
title: router.go should fail fast at startup if embedded SPA is missing index.html
finding: BUG-W144 was 'desktop binary ships without Vue SPA assets' — symptom was a directory listing instead of the app, because fs.Sub(staticFiles, 'static/v2') silently succeeded even when the directory was empty. The spaHandler closure falls back to index.html for unknown paths, but if index.html itself is missing, http.FileServer falls back to a directory listing. This ticket is the last one to touch that code path and is the right moment to add a startup check that fails loudly instead of silently serving a directory listing.
severity: significant
resolution: 'Added an fs.Stat(spaFS, ''index.html'') check immediately after fs.Sub in router.go. Panics at server startup with a clear actionable message: ''embedded SPA is missing index.html (run `just build-frontend`): ...''. This eliminates the BUG-W144 regression class going forward — any build that somehow fails to populate static/v2 will crash immediately at startup instead of silently serving a directory listing.'
status: addressed
---
