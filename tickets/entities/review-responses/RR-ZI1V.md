---
id: RR-ZI1V
type: review-response
title: Plan does not address TOCTOU in /api/open-file path containment
finding: 'Plan proposes filepath.Abs + EvalSymlinks + HasPrefix check. EvalSymlinks resolves at check time, but the file is then passed to the OS open command later. Between check and use, the path can be replaced with a symlink. On macOS/Linux this is a small window but exploitable. Also: filepath.Abs alone does not collapse `..` cleanly on all platforms — should use filepath.Clean explicitly. And the check `HasPrefix(abs, root+sep)` fails for `root` itself (the boundary case).'
severity: minor
resolution: 'Plan updated: use `filepath.Clean` then `filepath.Abs` then `filepath.EvalSymlinks`, then check `abs == root || strings.HasPrefix(abs, root+sep)`. TOCTOU window is acknowledged as an accepted residual risk because the open command runs synchronously milliseconds after the check, the user''s local FS is the trust boundary, and proper mitigation (file descriptor passing) is not portable across the open/xdg-open/explorer commands. Documented as a known limitation in docs/security.md.'
status: addressed
---
