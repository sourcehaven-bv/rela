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
// Sniffing (http.DetectContentType) is coarse and produces two GENERIC results
// rather than one: "application/octet-stream" for most non-text binaries, and
// "application/zip" for anything with a ZIP header — which includes every
// ZIP-container document format (docx, xlsx, odt, epub, …). We therefore allow
// both generic results and lean on the extension-mismatch check plus the
// explicit deny set to catch the dangerous cases, rather than trying to
// positively fingerprint every safe format.
var DefaultSafeMIMETypes = []string{
	"image/png",
	"image/jpeg",
	"image/gif",
	"image/webp",
	"application/pdf",
	"text/plain",
	"text/csv",
	"application/zip",          // archives and ZIP-container documents
	"application/octet-stream", // other binaries — sniffed generically
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
//
// The ZIP-based entries (".jar" onward) carry executables or macros, and their
// bytes sniff as the allowed "application/zip", so an explicit deny is the ONLY
// thing that stops them. Do not rely on the extension-mismatch check for these
// — it resolves the claim through the OS MIME database, which omits most of
// them on a minimal container image, and a missing claim skips the check
// entirely.
var deniedExtensions = map[string]bool{
	".svg": true, ".html": true, ".htm": true, ".xhtml": true,
	".js": true, ".mjs": true, ".exe": true, ".dll": true,
	".bat": true, ".cmd": true, ".com": true, ".sh": true,
	".ps1": true, ".scr": true, ".msi": true,
	// Executable / installer ZIP containers.
	".jar": true, ".war": true, ".ear": true,
	".apk": true, ".xpi": true, ".crx": true,
	".appx": true, ".msix": true, ".ipa": true,
	// Macro-enabled office documents. These carry VBA, so they are denied
	// rather than tolerated as ZIP containers. Named here for the same reason as
	// the executable containers above: a minimal image resolves most of them to
	// no claim at all, so leaving them to the mismatch check would accept them
	// there while rejecting them on a developer box.
	".docm": true, ".xlsm": true, ".pptm": true,
	".dotm": true, ".xltm": true, ".potm": true,
	".xlam": true, ".ppam": true, ".odb": true, ".oxt": true,
}

// zipContainerExtensions are the file extensions whose format is legitimately a
// ZIP archive, so a file sniffing as "application/zip" under one of them is not
// a polyglot. http.DetectContentType stops at the PK\x03\x04 header and cannot
// see the member names that distinguish a docx from an xlsx from a bare zip, so
// the extension is the only available signal and the mismatch check has nothing
// to compare.
//
// Keyed by EXTENSION rather than by the MIME type the extension claims, because
// the claim comes from mime.TypeByExtension, which reads the OS MIME database.
// Go's builtin table carries only .docx/.xlsx/.pptx/.apk/.zip/.pdf, so on a
// distroless or scratch image .odt/.ods/.odp/.odg/.epub resolve to nothing at
// all and a type-keyed set would silently stop matching them. Same reasoning as
// dataentry's appContentTypes: a deploy box's registry can both omit and
// override entries, so anything load-bearing is decided in rela's own source.
//
// This is an allowlist rather than a blanket "tolerate any claim over
// application/zip" because ZIP is also the container for executable formats —
// those are rejected outright by deniedExtensions above.
var zipContainerExtensions = map[string]bool{
	// OOXML (Microsoft Office). The macro-enabled variants (.docm, .xlsm,
	// .pptm) are deliberately absent: they are rejected rather than tolerated.
	".docx": true, ".xlsx": true, ".pptx": true,
	// OpenDocument (LibreOffice / OpenOffice), documents and templates.
	".odt": true, ".ods": true, ".odp": true, ".odg": true,
	".ott": true, ".ots": true, ".otp": true, ".otg": true,
	// EPUB
	".epub": true,
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

	// Normalized once here and threaded down: the deny set, the claim lookup and
	// the ZIP-container decision must all agree on what "the extension" is, and
	// three independent normalizations would drift.
	ext := strings.ToLower(filepath.Ext(pc.FileName))
	if deniedExtensions[ext] {
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
	if !extensionMatchesSniff(ext, sniffed) {
		return nil, ProcessInfo{}, Rejectedf(
			"file extension implies %q but content is %q", mimeForExt(ext), sniffed)
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

// mimeForExt returns the MIME type ext implies, or "" when the extension is
// unknown to the host. Used only by the mismatch check, so an unknown extension
// is permissive (no claim → no mismatch).
//
// ext MUST already be lowercased — [mimeProcessor.Process] normalizes it once
// and threads it down.
func mimeForExt(ext string) string {
	if ext == "" {
		return ""
	}
	return baseMIME(mime.TypeByExtension(ext))
}

// extensionMatchesSniff reports whether the format ext names is consistent with
// what the bytes sniffed as. It is the polyglot check (.jpg.php): an extension
// claiming one concrete type over the bytes of a different one is refused.
//
// An unknown extension makes no claim, so it matches anything — the host MIME
// database decides what is "unknown", which is why nothing security-critical may
// rest on this path. Formats that must be refused regardless are named in
// [deniedExtensions], which is consulted earlier and does not consult the host.
//
// ext MUST already be lowercased (see [mimeForExt]).
func extensionMatchesSniff(ext, sniffed string) bool {
	// A ZIP-container document (docx, odt, epub, …) legitimately sniffs as the
	// archive it is; its extension is the only thing that names the format, so
	// this is decided on ext alone rather than on the host-resolved claim.
	if sniffed == "application/zip" {
		return zipContainerExtensions[ext] || mimeForExt(ext) == "application/zip"
	}
	claimed := mimeForExt(ext)
	switch {
	case claimed == "":
		return true // no claim → nothing to contradict
	case claimed == sniffed:
		return true
	case sniffed == "application/octet-stream":
		// The sniffer recognized nothing at all, so it constrains the bytes not
		// at all and any claim survives it.
		return true
	case strings.HasPrefix(claimed, "text/") && sniffed == "text/plain":
		// text/* claims sniffing as text/plain are fine (csv, etc.).
		return true
	}
	return false
}
