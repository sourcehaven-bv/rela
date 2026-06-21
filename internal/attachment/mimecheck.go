package attachment

import (
	"bytes"
	"context"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

// readerFor re-offers buffered bytes as a reader.
func readerFor(data []byte) io.Reader { return bytes.NewReader(data) }

// DefaultSafeMIMETypes is the built-in `default-safe` allowlist: the sniffed
// MIME types accepted when a metamodel does not specify its own `allow` list.
// It deliberately EXCLUDES the active/script-carrying types that drive
// stored-XSS and code-execution on download — SVG (XML + script), HTML, and
// executables — per the OWASP File Upload Cheat Sheet.
//
// Sniffing (http.DetectContentType) is coarse: it reports the generic
// "application/octet-stream" for most non-text binaries (office docs, many
// archives). We therefore allow octet-stream and lean on the
// extension-mismatch check plus the explicit deny set to catch the dangerous
// cases, rather than trying to positively fingerprint every safe format.
var DefaultSafeMIMETypes = []string{
	"image/png",
	"image/jpeg",
	"image/gif",
	"image/webp",
	"application/pdf",
	"text/plain",
	"text/csv",
	"application/zip",
	"application/octet-stream", // office docs, archives — sniffed generically
}

// deniedMIMETypes are never allowed regardless of the allowlist: they execute
// active content when served. The check is defensive — these also fail the
// allowlist — but an explicit deny makes the intent legible and survives an
// operator widening `allow`.
var deniedMIMETypes = map[string]bool{
	"image/svg+xml":          true,
	"text/html":              true,
	"application/xhtml+xml":  true,
	"application/javascript": true,
	"text/javascript":        true,
}

// deniedExtensions are file extensions rejected outright (active content /
// executables), independent of sniffed type — defends against a polyglot that
// sniffs as an image but is named ".svg"/".html"/".exe".
var deniedExtensions = map[string]bool{
	".svg": true, ".html": true, ".htm": true, ".xhtml": true,
	".js": true, ".mjs": true, ".exe": true, ".dll": true,
	".bat": true, ".cmd": true, ".com": true, ".sh": true,
	".ps1": true, ".scr": true, ".msi": true,
}

// mimeProcessor enforces the MIME allowlist against the SNIFFED content type
// (never the client-supplied header) and rejects sniff↔extension mismatches.
// It is pure input validation — it does not mutate bytes — so it returns the
// reader unchanged after sniffing the prefix.
type mimeProcessor struct {
	// allow is the set of accepted sniffed MIME base types. Empty means use
	// DefaultSafeMIMETypes.
	allow []string
}

// newMIMEProcessor builds a validator from an allowlist. A nil/empty list uses
// the default-safe preset. A list whose only element is "default-safe" is also
// treated as the preset (so a metamodel can name it explicitly).
func newMIMEProcessor(allow []string) *mimeProcessor {
	if len(allow) == 0 || (len(allow) == 1 && allow[0] == "default-safe") {
		return &mimeProcessor{allow: DefaultSafeMIMETypes}
	}
	return &mimeProcessor{allow: allow}
}

// NeedsFullFile returns true: the seam buffers the upload so we can sniff its
// head and still hand the complete bytes to the store. (Sniffing alone needs
// only 512 bytes, but the processor must pass every byte through.)
func (p *mimeProcessor) NeedsFullFile() bool { return true }

// Process sniffs the content type, checks it against the allowlist and the
// extension, and passes the bytes through unchanged on success.
func (p *mimeProcessor) Process(
	_ context.Context, pc ProcessContext, r io.Reader,
) (io.Reader, ProcessInfo, error) {
	// The seam has buffered the bytes, so r is a *bytes.Reader-style reader we
	// can read fully and re-offer. Read once; sniff the prefix; re-wrap.
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, ProcessInfo{}, err
	}

	if ext := strings.ToLower(filepath.Ext(pc.FileName)); deniedExtensions[ext] {
		return nil, ProcessInfo{}, Rejectedf("file type %q is not allowed", ext)
	}

	sniffed := baseMIME(http.DetectContentType(data))
	if deniedMIMETypes[sniffed] {
		return nil, ProcessInfo{}, Rejectedf("content type %q is not allowed", sniffed)
	}
	if !p.allows(sniffed) {
		return nil, ProcessInfo{}, Rejectedf("content type %q is not in the allowed list", sniffed)
	}
	// Sniff↔extension mismatch (polyglot / .jpg.php): the extension claims a
	// concrete type but the bytes sniff as something incompatible.
	if claimed := mimeForExt(pc.FileName); claimed != "" && !mimeCompatible(claimed, sniffed) {
		return nil, ProcessInfo{}, Rejectedf(
			"file extension implies %q but content is %q", claimed, sniffed)
	}

	return readerFor(data), ProcessInfo{}, nil
}

func (p *mimeProcessor) allows(sniffed string) bool {
	for _, a := range p.allow {
		if baseMIME(a) == sniffed {
			return true
		}
	}
	return false
}

// baseMIME strips parameters and lowercases ("text/plain; charset=utf-8" →
// "text/plain").
func baseMIME(s string) string {
	if i := strings.IndexByte(s, ';'); i >= 0 {
		s = s[:i]
	}
	return strings.ToLower(strings.TrimSpace(s))
}

// mimeForExt returns the MIME type the file extension implies, or "" when the
// extension is unknown. Used only for the mismatch check, so an unknown
// extension is permissive (no claim → no mismatch).
func mimeForExt(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	if ext == "" {
		return ""
	}
	return baseMIME(mime.TypeByExtension(ext))
}

// mimeCompatible reports whether a sniffed type is acceptable for an
// extension-claimed type. Exact match always passes. A claimed concrete type
// that sniffs as the generic octet-stream is tolerated (the sniffer can't
// fingerprint many valid formats); but a claimed image that sniffs as a
// *different concrete* type is a mismatch.
func mimeCompatible(claimed, sniffed string) bool {
	if claimed == sniffed {
		return true
	}
	if sniffed == "application/octet-stream" {
		return true
	}
	// text/* claims sniffing as text/plain are fine (csv, etc.).
	if strings.HasPrefix(claimed, "text/") && sniffed == "text/plain" {
		return true
	}
	return false
}
