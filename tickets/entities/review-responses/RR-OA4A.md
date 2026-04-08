---
id: RR-OA4A
type: review-response
title: 'loadUserPalette silently swallows parse errors — old `dark: auto` files are silently wiped'
finding: |-
    internal/dataentry/app.go:288 `loadUserPalette` does `if err := yaml.Unmarshal(data, &p); err != nil { return nil }` — both file-not-found and parse-error are collapsed to nil. With this PR, any existing `.rela/palette.yaml` containing `dark: auto` (the previous default) will fail `DarkMode.UnmarshalYAML` and be silently dropped on load. The Settings page then renders defaults. The next click on Save Palette overwrites the user's customised palette.yaml with the framework defaults — the original is gone unless the user has version control on `.rela/`. The ticket itself acknowledges this migration risk (`Existing `.rela/palette.yaml` files with `dark: auto` will fail to load`) but the load path was not updated to surface the failure.

    Fixes: (a) distinguish ENOENT from parse error, (b) on parse error, log loudly via the toast/notification system AND return a sentinel `&PaletteConfig{}` rather than nil so saves don't proceed silently, OR (c) auto-migrate `dark: auto` → drop the field on first read and rewrite. I'd vote for (c) since it's a 5-line fix and matches the rest of the migration system in `internal/migration/`.

    Bonus: the same swallowing happens in the watcher reload path (`watcher.go:194`) — the user could be editing happily in another tool, paste in `dark: auto`, and the watcher silently reverts to defaults with no UI feedback.
severity: critical
resolution: 'Reworked `loadUserPalette` in `internal/dataentry/app.go` to return `(*PaletteConfig, error)`. ENOENT returns (nil, nil); any other read or YAML parse error returns a wrapped error with a clear migration hint about `dark: auto`. Updated callers: NewApp now fails to start with a clear error, and the watcher reload path keeps the previous palette and logs a warning instead of clobbering it. Added 3 new unit tests in handlers_api_test.go (`TestLoadUserPalette`) covering missing file, malformed legacy `dark: auto`, and the happy path.'
status: addressed
---
