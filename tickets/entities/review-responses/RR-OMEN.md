---
id: RR-OMEN
type: review-response
title: writeThemeImportError default falls to 400 silently
finding: theme_package.go::writeThemeImportError — Switch is exhaustive on current sentinels; any new sentinel goes to 400 silently. Add a slog.Warn (or test that fails on unmapped sentinel) so misroutes are visible.
severity: nit
resolution: 'writeThemeImportError now exhaustively lists all sentinels in the switch with the right HTTP status mapping. The default branch (unrecognized errors) emits a slog.Warn (''theme import: unmapped parse error'') so future-added sentinels surface in logs even if they fall to the conservative 400 default.'
status: addressed
---
