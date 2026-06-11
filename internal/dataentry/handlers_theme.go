package dataentry

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// maxLogoUploadBytes is the hard cap on the multipart request body. It
// is intentionally larger than MaxUserLogoBytes by a generous headroom
// so a logo right at the size cap can still arrive with its multipart
// envelope (boundary lines, Content-Disposition with long UTF-8
// filenames, extra form fields). The precise content check happens
// after parsing, on the file bytes alone — this constant just bounds
// the worst-case memory pressure of a single request.
const maxLogoUploadBytes = MaxUserLogoBytes + 16*1024

// handleAPIThemeLogo routes /api/v1/_theme/logo by HTTP method.
func (a *App) handleAPIThemeLogo(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.handleAPIGetThemeLogo(w, r)
	case http.MethodPut, http.MethodPost:
		a.handleAPIPutThemeLogo(w, r)
	case http.MethodDelete:
		a.handleAPIDeleteThemeLogo(w, r)
	default:
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleAPIGetThemeLogo serves the user-uploaded logo bytes. Responses
// include CSP sandbox + nosniff + immutable cache headers so user-supplied
// SVG can't be coerced into a script-execution context, even on direct
// navigation, and so cache-busting works via the URL alone.
func (a *App) handleAPIGetThemeLogo(w http.ResponseWriter, _ *http.Request) {
	s := a.State()
	if s.UserLogoExt == "" || len(s.UserLogoBytes) == 0 {
		writeJSONError(w, http.StatusNotFound, "no logo set")
		return
	}
	ct := logoContentType(s.UserLogoExt)
	if ct == "" {
		// Should never happen — saveUserLogo validates the extension
		// before persisting, and loadUserLogo treats unknown values as
		// "not set". Treat as a server-side bug rather than serving
		// bytes with no Content-Type.
		writeJSONError(w, http.StatusInternalServerError, "logo has unknown extension")
		return
	}
	h := w.Header()
	h.Set("Content-Type", ct)
	h.Set("X-Content-Type-Options", "nosniff")
	// CSP sandbox neutralizes scripts on direct navigation; frame-ancestors
	// 'none' (and X-Frame-Options as a belt) prevents the response from
	// being framed by an attacker page even if origin checks are loosened.
	h.Set("Content-Security-Policy", "sandbox; frame-ancestors 'none'")
	h.Set("X-Frame-Options", "DENY")
	// The URL contains a content hash — any update produces a different
	// URL, so the cached response can never go stale.
	h.Set("Cache-Control", "public, max-age=86400, immutable")
	_, _ = w.Write(s.UserLogoBytes)
}

// handleAPIPutThemeLogo accepts a multipart upload, validates it, and
// persists the bytes + extension under writeMu via mutateState so the
// AppState snapshot is coherent for concurrent readers.
func (a *App) handleAPIPutThemeLogo(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxLogoUploadBytes)

	if err := r.ParseMultipartForm(maxLogoUploadBytes); err != nil {
		// http.MaxBytesReader writes a *http.MaxBytesError when the
		// limit is exceeded; surface that as 413 rather than 400.
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeLogoTooLarge(w)
			return
		}
		writeJSONError(w, http.StatusBadRequest, "invalid multipart body: "+err.Error())
		return
	}

	file, _, err := r.FormFile("logo")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, `missing form field "logo"`)
		return
	}
	defer func() { _ = file.Close() }()

	bytes, err := io.ReadAll(file)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeLogoTooLarge(w)
			return
		}
		writeJSONError(w, http.StatusBadRequest, "failed to read logo: "+err.Error())
		return
	}
	if len(bytes) > MaxUserLogoBytes {
		writeLogoTooLarge(w)
		return
	}
	if len(bytes) == 0 {
		writeJSONError(w, http.StatusBadRequest, "empty logo")
		return
	}

	mime := sniffLogoMime(bytes)
	ext := logoExtForMime(mime)
	if ext == "" {
		writeJSONError(w, http.StatusBadRequest,
			fmt.Sprintf("unsupported format: %s (accepted: image/png, image/jpeg, image/svg+xml, image/webp)", mime))
		return
	}

	hash := hashLogoBytes(bytes)

	var saveErr error
	ctx := r.Context()
	a.mutateState(func(s *AppState) {
		if err := a.saveUserLogo(ctx, bytes, ext); err != nil {
			// On failure the snapshot copy is left untouched and
			// mutateState republishes a bytewise-identical pointer.
			// Cheap (one struct copy) and keeps the path simple — do
			// not "optimize" by skipping the publish.
			saveErr = err
			return
		}
		s.UserLogoBytes = bytes
		s.UserLogoExt = ext
		s.UserLogoHash = hash
	})
	if saveErr != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to save logo: "+saveErr.Error())
		return
	}

	writeJSON(w, map[string]any{
		"ok":      true,
		"logoUrl": logoURLForHash(hash),
	})
}

// handleAPIDeleteThemeLogo clears the persisted logo. Idempotent: if no
// logo is set the call still succeeds (the on-disk Delete is itself
// idempotent), so callers that don't track current logo state can just
// hit the endpoint.
func (a *App) handleAPIDeleteThemeLogo(w http.ResponseWriter, r *http.Request) {
	var deleteErr error
	ctx := r.Context()
	a.mutateState(func(s *AppState) {
		if err := a.deleteUserLogo(ctx); err != nil {
			deleteErr = err
			return
		}
		s.UserLogoBytes = nil
		s.UserLogoExt = ""
		s.UserLogoHash = ""
	})
	if deleteErr != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to delete logo: "+deleteErr.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// sniffLogoMime detects the content type from the leading bytes.
// http.DetectContentType returns SVG payloads as "text/xml" or
// "text/plain" depending on the prologue; normalize those to image/svg+xml
// when the body looks like SVG so users don't have to worry about a BOM
// or a missing XML declaration.
func sniffLogoMime(b []byte) string {
	mime := http.DetectContentType(b)
	if i := strings.IndexByte(mime, ';'); i >= 0 {
		mime = strings.TrimSpace(mime[:i])
	}
	if mime == "image/svg+xml" {
		return mime
	}
	if (mime == "text/xml" || mime == "text/plain" || mime == "application/xml") && looksLikeSVG(b) {
		return "image/svg+xml"
	}
	return mime
}

// looksLikeSVG checks whether the byte stream is an SVG document — i.e.
// whether the first real element is <svg, after optional whitespace, a
// UTF-8 BOM, an XML prologue, a doctype declaration, or comments.
//
// A loose contains("<svg") would let polyglots through ("<?xml ?><!--
// junk --><div><svg>...</svg><script>...</script>"). We rely on the
// browser <img> sandbox + CSP for ultimate safety, but a tight server
// sniff keeps the trust boundary simple to reason about.
func looksLikeSVG(b []byte) bool {
	const window = 1024
	if len(b) > window {
		b = b[:window]
	}
	s := string(b)
	// Strip a UTF-8 BOM (U+FEFF) if present.
	s = strings.TrimPrefix(s, "\uFEFF")
	for {
		s = strings.TrimLeft(s, " \t\r\n")
		if s == "" {
			return false
		}
		if s[0] != '<' {
			return false
		}
		lower := strings.ToLower(s)
		switch {
		case strings.HasPrefix(lower, "<svg"):
			// First real element is <svg — trailing chars must be
			// whitespace, '/', or '>' so we don't mistake "<svgfoo".
			rest := s[4:]
			if rest == "" {
				return true
			}
			r := rest[0]
			return r == ' ' || r == '\t' || r == '\r' || r == '\n' || r == '/' || r == '>'
		case strings.HasPrefix(lower, "<?xml"),
			strings.HasPrefix(lower, "<!doctype"),
			strings.HasPrefix(lower, "<!--"):
			// Skip prologue / doctype / comment by finding its
			// terminator and continuing.
			term := ">"
			if strings.HasPrefix(lower, "<!--") {
				term = "-->"
			}
			i := strings.Index(s, term)
			if i < 0 {
				return false
			}
			s = s[i+len(term):]
			continue
		default:
			return false
		}
	}
}

// logoURLForHash returns the public URL for the current logo, including
// the cache-busting query parameter.
func logoURLForHash(hash string) string {
	return "/api/v1/_theme/logo?v=" + hash
}

// writeLogoTooLarge sends a structured 413 carrying the server-side
// limit so the SPA can render an accurate error message without
// duplicating the constant.
func writeLogoTooLarge(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusRequestEntityTooLarge)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error":    fmt.Sprintf("logo too large: max %d bytes", MaxUserLogoBytes),
		"maxBytes": MaxUserLogoBytes,
	})
}
