package dataentry

import (
	"crypto/sha256"
	"encoding/hex"
)

// User-uploaded theme assets are stored under .rela/theme/. The bytes
// file ("theme/logo") is opaque; the sidecar ("theme/logo.ext") records
// the inferred extension so the GET handler can set Content-Type without
// re-sniffing on every request.
const (
	userLogoFile    = "theme/logo"
	userLogoExtFile = "theme/logo.ext"

	// MaxUserLogoBytes caps user-uploaded logos. Sidebar logos render at
	// ~28px tall; 256 KiB is generous for that display size.
	MaxUserLogoBytes = 256 << 10
)

// allowedLogoExts is the canonical extension set we persist. The PUT
// handler maps sniffed mime types into one of these; no other value is
// ever written.
var allowedLogoExts = map[string]struct{}{
	"png":  {},
	"jpeg": {},
	"svg":  {},
	"webp": {},
}

// logoContentType returns the response Content-Type for a stored logo.
// Returns "" for unknown extensions so callers can decide whether to
// serve the bytes (we choose not to — see handleAPIGetThemeLogo).
func logoContentType(ext string) string {
	switch ext {
	case "png":
		return "image/png"
	case "jpeg":
		return "image/jpeg"
	case "svg":
		return "image/svg+xml"
	case "webp":
		return "image/webp"
	}
	return ""
}

// logoExtForMime maps a sniffed mime type to the canonical extension we
// persist. Returns "" for any mime not in the allowlist.
func logoExtForMime(mime string) string {
	switch mime {
	case "image/png":
		return "png"
	case "image/jpeg":
		return "jpeg"
	case "image/svg+xml":
		// http.DetectContentType reports SVG as "image/svg+xml" or
		// "text/xml; charset=utf-8" depending on the prologue. The PUT
		// handler normalizes the latter before calling us.
		return "svg"
	case "image/webp":
		return "webp"
	}
	return ""
}

// hashLogoBytes returns a short content hash used as a cache-busting
// query parameter. 12 hex chars (48 bits) is far more than enough for a
// per-workspace single-logo slot: a collision would mean a stale cached
// image gets served until the cache expires (not a no-op), but the
// probability of that on a single-logo workspace is effectively zero.
func hashLogoBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:6])
}

// LogoURL returns the public URL for the user-uploaded logo (including
// the cache-busting query parameter), or nil when no logo is set. The
// single source of truth for handlers that surface the URL to the SPA,
// so a future change (signing, expiry, alternate cache strategy) lands
// in one place.
func (s *AppState) LogoURL() *string {
	if s.UserLogoHash == "" {
		return nil
	}
	u := logoURLForHash(s.UserLogoHash)
	return &u
}
