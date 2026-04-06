---
id: RR-NUT7
type: review-response
title: Config reload path needs palette rebuild
finding: The file watcher in watcher.go rebuilds styleMap on config reload (line 193) but the plan doesn't mention adding palette resolution to the same reload path. If palette is only resolved in NewApp(), config hot-reload won't pick up palette changes — user would need to restart the server.
severity: minor
resolution: 'Plan updated: palette resolution added to onReload() path in watcher.go alongside buildStyleMap(). Also reload user palette from disk on config change.'
status: addressed
---
