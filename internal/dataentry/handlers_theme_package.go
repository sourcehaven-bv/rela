package dataentry

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
)

// maxThemeUploadBytes is the hard cap on the multipart request body for
// theme imports. Generous envelope above ThemePackageMaxBytes mirrors
// maxLogoUploadBytes — keeps both upload paths in lockstep.
const maxThemeUploadBytes = ThemePackageMaxBytes + 16*1024

// APIThemeImportResponse is the typed shape of POST /_theme/import.
// Field names mirror the analogous fields in APISettingsData so the
// frontend stays aligned with the rest of the JSON surface.
type APIThemeImportResponse struct {
	Palette dataentryconfig.PaletteConfig `json:"palette"`
	LogoURL string                        `json:"logoUrl,omitempty"`
}

// handleAPIThemeExport returns the user's current palette + logo
// bundled as a `.relatheme` zip download. Always emits a manifest;
// includes the logo file when one is set.
func (a *App) handleAPIThemeExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	s := a.State()
	manifest := buildExportManifest(s)
	zipBytes, err := buildThemeZip(manifest, s.UserLogoBytes, s.UserLogoExt)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to build theme package: "+err.Error())
		return
	}
	filename := safeThemeFilename(manifest.Name) + ".relatheme"
	h := w.Header()
	h.Set("Content-Type", "application/zip")
	h.Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	// Exports reflect current state — never cache.
	h.Set("Cache-Control", "no-store")
	_, _ = w.Write(zipBytes)
}

// handleAPIThemeImport accepts a `.relatheme` upload, persists the
// logo if one was bundled, and returns the parsed palette JSON for the
// frontend to stage in the existing palette editor. The palette is NOT
// auto-saved; matches the existing palette UX where colors persist
// only on explicit Save.
func (a *App) handleAPIThemeImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxThemeUploadBytes)
	if err := r.ParseMultipartForm(ThemePackageMaxBytes + 16*1024); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeThemeTooLarge(w)
			return
		}
		writeJSONError(w, http.StatusBadRequest, "invalid multipart body: "+err.Error())
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, `missing form field "file"`)
		return
	}
	defer func() { _ = file.Close() }()

	raw, err := io.ReadAll(file)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeThemeTooLarge(w)
			return
		}
		writeJSONError(w, http.StatusBadRequest, "failed to read upload: "+err.Error())
		return
	}

	pkg, err := parseThemePackage(raw)
	if err != nil {
		writeThemeImportError(w, err)
		return
	}

	logoURL := ""
	if pkg.Logo != nil {
		hash := hashLogoBytes(pkg.Logo.Bytes)
		var saveErr error
		ctx := r.Context()
		a.mutateState(func(s *AppState) {
			if err := a.userState.saveUserLogo(ctx, pkg.Logo.Bytes, pkg.Logo.Ext); err != nil {
				saveErr = err
				return
			}
			s.UserLogoBytes = pkg.Logo.Bytes
			s.UserLogoExt = pkg.Logo.Ext
			s.UserLogoHash = hash
		})
		if saveErr != nil {
			writeJSONError(w, http.StatusInternalServerError, "failed to save logo: "+saveErr.Error())
			return
		}
		logoURL = logoURLForHash(hash)
	}

	writeJSON(w, APIThemeImportResponse{
		Palette: pkg.Manifest.PaletteConfig,
		LogoURL: logoURL,
	})
}

// buildExportManifest composes the manifest written into the zip,
// drawing the palette from user state when available and falling back
// to the project palette otherwise.
func buildExportManifest(s *AppState) *dataentryconfig.ThemeManifest {
	m := &dataentryconfig.ThemeManifest{
		Name:    s.Cfg.App.Name,
		Version: "1.0.0",
	}
	if m.Name == "" {
		m.Name = "Theme"
	}

	switch {
	case s.UserPalette != nil:
		m.PaletteConfig = *s.UserPalette
	case s.Cfg.Palette != nil:
		m.PaletteConfig = *s.Cfg.Palette
	}

	if s.UserLogoExt != "" {
		m.Logo = "logo." + s.UserLogoExt
	}
	return m
}

// buildThemeZip writes a `.relatheme` archive into a buffer. The
// manifest is required; logo bytes are written under `logo.<ext>` only
// when both are present.
func buildThemeZip(manifest *dataentryconfig.ThemeManifest, logoBytes []byte, logoExt string) ([]byte, error) {
	manifestYAML, err := yaml.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("marshal manifest: %w", err)
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	mf, err := zw.Create(themeManifestEntry)
	if err != nil {
		return nil, fmt.Errorf("create manifest entry: %w", err)
	}
	if _, err := mf.Write(manifestYAML); err != nil {
		return nil, fmt.Errorf("write manifest entry: %w", err)
	}
	if logoExt != "" && len(logoBytes) > 0 {
		lf, err := zw.Create(themeLogoPrefix + logoExt)
		if err != nil {
			return nil, fmt.Errorf("create logo entry: %w", err)
		}
		if _, err := lf.Write(logoBytes); err != nil {
			return nil, fmt.Errorf("write logo entry: %w", err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("close zip: %w", err)
	}
	return buf.Bytes(), nil
}

var unsafeFilenameRe = regexp.MustCompile(`[^A-Za-z0-9_-]+`)

// safeThemeFilename derives a safe download filename from the
// manifest's Name. Browsers will let users override it via "Save as..."
// so this is purely cosmetic.
func safeThemeFilename(name string) string {
	cleaned := unsafeFilenameRe.ReplaceAllString(name, "_")
	cleaned = strings.Trim(cleaned, "_")
	if cleaned == "" {
		return "theme"
	}
	if len(cleaned) > 64 {
		cleaned = cleaned[:64]
	}
	return cleaned
}

// writeThemeImportError translates parseThemePackage sentinel errors
// into the right HTTP status + JSON shape. Unrecognized parse errors
// fall through to 400 with a slog.Warn so a misroute is visible in
// logs (we want to notice if a new sentinel is added without being
// wired in here).
func writeThemeImportError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errLogoTooLarge),
		errors.Is(err, errZipUncompressed),
		errors.Is(err, errZipBomb):
		writeJSONError(w, http.StatusRequestEntityTooLarge, err.Error())
	case errors.Is(err, errNotAZip),
		errors.Is(err, errMissingManifest),
		errors.Is(err, errInvalidManifest),
		errors.Is(err, errMissingLogo),
		errors.Is(err, errZipPathTraversal),
		errors.Is(err, errDuplicateEntry):
		writeJSONError(w, http.StatusBadRequest, err.Error())
	default:
		// New sentinel added without being wired in here — still a
		// 400 (least-privilege default), but log so it gets noticed.
		slog.Warn("theme import: unmapped parse error", "err", err)
		writeJSONError(w, http.StatusBadRequest, err.Error())
	}
}

// writeThemeTooLarge mirrors writeLogoTooLarge but for theme packages.
// Includes the cap so the SPA can surface an accurate message.
func writeThemeTooLarge(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusRequestEntityTooLarge)
	_, _ = fmt.Fprintf(w,
		`{"error":"theme package too large: max %d bytes","maxBytes":%d}`,
		ThemePackageMaxBytes, ThemePackageMaxBytes,
	)
}
