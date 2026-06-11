package dataentry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
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

// loadUserLogo reads the persisted logo bytes and extension. Returns
// (nil, "", nil) when no logo is set. A sidecar file present without
// matching bytes (or vice versa) is treated as "no logo set" so a
// half-written state during a crash doesn't trip up the boot path.
//
// Returns a non-nil error only when the kv layer reports an unexpected
// failure (corrupt filesystem, permission denied) — callers should
// surface those instead of silently masking them.
//
// Concurrency: NOT safe to call in parallel with saveUserLogo or
// deleteUserLogo (the bytes/sidecar pair is read non-atomically). Boot
// path calls this before publishing the App; future reload paths must
// hold writeMu (i.e. invoke from inside mutateState).
//
//nolint:gocritic // unnamedResult: handler-style (bytes, ext, err) is clearer than naming for two callers
func (a *App) loadUserLogo() ([]byte, string, error) {
	if a.kv == nil {
		return nil, "", nil
	}
	ctx := context.Background()

	logoBytes, err := a.kv.Get(ctx, userLogoFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || os.IsNotExist(err) {
			return nil, "", nil
		}
		return nil, "", fmt.Errorf("read %s: %w", userLogoFile, err)
	}

	extBytes, err := a.kv.Get(ctx, userLogoExtFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || os.IsNotExist(err) {
			// Bytes present but sidecar missing — treat as not-set.
			// The user can re-upload to recover; we don't proactively
			// clean up so a future fix can recover the bytes.
			return nil, "", nil
		}
		return nil, "", fmt.Errorf("read %s: %w", userLogoExtFile, err)
	}

	ext := string(extBytes)
	if _, ok := allowedLogoExts[ext]; !ok {
		// Sidecar contains an unknown extension — treat as not-set.
		// Same reasoning as the missing-sidecar case.
		return nil, "", nil
	}

	return logoBytes, ext, nil
}

// saveUserLogo persists the bytes and extension. Caller must hold
// writeMu (i.e. invoke from inside mutateState) so the bytes/sidecar
// pair cannot be observed half-written by a concurrent reload.
func (a *App) saveUserLogo(ctx context.Context, bytes []byte, ext string) error {
	if a.kv == nil {
		return errors.New("kv not configured")
	}
	if _, ok := allowedLogoExts[ext]; !ok {
		return fmt.Errorf("invalid logo extension %q", ext)
	}
	if err := a.kv.Put(ctx, userLogoFile, bytes); err != nil {
		return fmt.Errorf("write %s: %w", userLogoFile, err)
	}
	if err := a.kv.Put(ctx, userLogoExtFile, []byte(ext)); err != nil {
		return fmt.Errorf("write %s: %w", userLogoExtFile, err)
	}
	return nil
}

// deleteUserLogo removes both the bytes file and the sidecar. Idempotent;
// missing files are not errors.
func (a *App) deleteUserLogo(ctx context.Context) error {
	if a.kv == nil {
		return nil
	}
	if err := a.kv.Delete(ctx, userLogoFile); err != nil {
		return fmt.Errorf("delete %s: %w", userLogoFile, err)
	}
	if err := a.kv.Delete(ctx, userLogoExtFile); err != nil {
		return fmt.Errorf("delete %s: %w", userLogoExtFile, err)
	}
	return nil
}
