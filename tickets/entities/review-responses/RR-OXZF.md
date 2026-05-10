---
id: RR-OXZF
type: review-response
title: DRY logoUrl construction across _settings and _sidebar
finding: Both handlers_api.go::handleAPIGetSettings and api_v1.go::handleV1Sidebar duplicate `if s.UserLogoHash != "" { u := logoURLForHash(s.UserLogoHash); … }`. Lift into AppState.LogoURL() *string so future changes (signing, expiry) only touch one site.
severity: significant
resolution: Lifted the URL construction into AppState.LogoURL() in theme_logo.go. Both handlers_api.go::handleAPIGetSettings and api_v1.go::handleV1Sidebar now call s.LogoURL() — single source of truth for any future signing/expiry change.
status: addressed
---
