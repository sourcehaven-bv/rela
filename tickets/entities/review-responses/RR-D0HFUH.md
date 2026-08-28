---
id: RR-D0HFUH
type: review-response
title: app_editor_dist has the same empty-embed failure mode and is unguarded (.gitkeep removes the build-error safety net)
finding: 'internal/dataentry/apps_editor.go embeds `all:app_editor_dist`, built by the same npm run build. Verified .gitignore:57-58 ignores the dir but keeps a committed .gitkeep. That is strictly more dangerous than static/v2: an entirely empty embed dir is a hard Go build error, but the .gitkeep guarantees the glob always matches, so a missing editor bundle produces no build error at all and appEditorAsset silently serves a dead editor. Extend the tree check to assert app_editor_dist/rela-editor.js is present and non-empty.'
severity: significant
resolution: The 'Verify SPA build output' step now also asserts internal/dataentry/app_editor_dist/rela-editor.js is present and non-empty, with a comment explaining why this one is more dangerous than static/v2 (the committed .gitkeep means the embed glob always matches, so a missing bundle is not even a build error). Verified the bundle is produced by the same npm run build (382789 bytes after just build-frontend).
status: addressed
---
