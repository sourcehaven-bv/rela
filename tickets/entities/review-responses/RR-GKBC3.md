---
id: RR-GKBC3
type: review-response
title: 'Significant: isInsideProject missed symlinks, macOS /var→/private/var, Windows case'
finding: 'cranky-code-reviewer #8: RR-242DF mitigation was broken on the exact platforms most likely to trigger it. A user''s symlinked key, a project under /var/folders on macOS, or a mixed-case Windows path would skip the warning.'
severity: significant
resolution: isInsideProject now runs filepath.EvalSymlinks on both sides (falls back to Abs result on error) and lowercases `rel` on Windows so the HasPrefix check is case-insensitive. See internal/cli/keys.go.
status: addressed
---
