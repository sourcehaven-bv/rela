---
id: RR-0SRDNX
type: review-response
title: Palette, logo and theme-import writes stay ungated, even under --read-only
finding: PUT /_palette, PUT/POST/DELETE /_theme/logo and POST /_theme/import write state shared by every principal, the same class as the settings defaults, with no permission check and no read-only refusal. (rela-security-reviewer, significant; cranky-code-reviewer raised the palette as minor)
severity: significant
resolution: All five writes go through appearanceHandler.refuseSettingsEdit, which applies PermitsProjectWrite with settings:edit (no policy allows, read-only refuses, policy asks the holder). New tests TestSavePalette_Gate, TestThemeLogo_Gate and TestThemeImport_Gate; the palette, logo and theme-import decl-header goldens now record 403 for non-admin identities. The guide lists every settings-page save under settings:edit.
status: addressed
---
