---
id: RR-YBW8
type: review-response
title: SettingsView duplicates 256 KiB constant from server
finding: frontend/src/views/SettingsView.vue duplicates MAX_LOGO_BYTES = 256 * 1024 from the server's MaxUserLogoBytes. If the server bumps the limit, the client error message lies until someone updates both. Either include the server max in the 413 response and surface it on rejection, or at minimum cross-reference with a comment.
severity: minor
resolution: Backend 413 response now carries the authoritative `maxBytes` (handlers_theme.go::writeLogoTooLarge). Frontend api/theme.ts surfaces it on LogoUploadError; SettingsView soft-checks at 256 KiB (renamed SOFT_MAX_LOGO_BYTES with a comment pointing at the server) but the toast on a real 413 reflects the server's number.
status: addressed
---
