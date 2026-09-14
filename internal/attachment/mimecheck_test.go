package attachment

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"sort"
	"strings"
	"testing"
)

// pngBytes is a minimal valid PNG header that http.DetectContentType reports as
// image/png.
var pngBytes = []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0}

// gifBytes sniffs as image/gif.
var gifBytes = []byte("GIF89a\x00\x00\x00\x00")

func runMIME(t *testing.T, allow []string, fileName string, data []byte) error {
	t.Helper()
	_, _, err := newMIMEProcessor(allow).Process(
		context.Background(), ProcessContext{FileName: fileName}, bytes.NewReader(data))
	return err
}

func TestMIME_AllowsSafeType(t *testing.T) {
	if err := runMIME(t, nil, "logo.png", pngBytes); err != nil {
		t.Errorf("png should be allowed by default-safe: %v", err)
	}
}

func TestMIME_PassesBytesUnchanged(t *testing.T) {
	out, _, err := newMIMEProcessor(nil).Process(
		context.Background(), ProcessContext{FileName: "a.png"}, bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	got, _ := io.ReadAll(out)
	if !bytes.Equal(got, pngBytes) {
		t.Error("MIME validator must pass bytes through unchanged")
	}
}

func TestMIME_BlocksByExtension(t *testing.T) {
	// Even if the bytes sniff as an image, a dangerous extension is rejected.
	for _, name := range []string{"x.svg", "x.html", "x.exe", "payload.js"} {
		if err := runMIME(t, nil, name, pngBytes); !errors.Is(err, ErrRejected) {
			t.Errorf("%s should be rejected by extension, got %v", name, err)
		}
	}
}

func TestMIME_BlocksSVGContent(t *testing.T) {
	svg := []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)
	// Named .txt to dodge the extension check — content sniffing must still
	// catch svg/html. http.DetectContentType reports text/xml or text/html
	// here; either way it is not in the allowlist.
	if err := runMIME(t, nil, "sneaky.txt", svg); !errors.Is(err, ErrRejected) {
		t.Errorf("svg/xml content should be rejected, got %v", err)
	}
}

func TestMIME_RejectsSniffExtensionMismatch(t *testing.T) {
	// Named .png but the bytes are a GIF → mismatch.
	if err := runMIME(t, nil, "fake.png", gifBytes); !errors.Is(err, ErrRejected) {
		t.Errorf("png-named gif should be rejected as a mismatch, got %v", err)
	}
}

func TestMIME_RejectsTypeNotInAllowlist(t *testing.T) {
	// gif is in the default-safe list; narrow to png-only and it should reject.
	if err := runMIME(t, []string{"image/png"}, "x.gif", gifBytes); !errors.Is(err, ErrRejected) {
		t.Errorf("gif should be rejected when allowlist is png-only, got %v", err)
	}
}

func TestMIME_DefaultSafePresetByName(t *testing.T) {
	// An explicit "default-safe" name resolves to the preset.
	if err := runMIME(t, []string{"default-safe"}, "a.png", pngBytes); err != nil {
		t.Errorf("default-safe by name should allow png: %v", err)
	}
}

// zipBytes builds a real ZIP archive. Every ZIP-container document format
// (docx, odt, epub, …) is one of these underneath, and http.DetectContentType
// reports "application/zip" for all of them — it stops at the PK\x03\x04 header
// and never reads the member names, so the archive's contents cannot affect any
// verdict under test.
func zipBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("content.xml")
	if err != nil {
		t.Fatalf("zip create: %v", err)
	}
	if _, err := w.Write([]byte(`<?xml version="1.0"?><x/>`)); err != nil {
		t.Fatalf("zip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

// TestMIME_AllowsZipContainerDocuments pins the fix for BUG-TDE1QO: a docx is a
// ZIP, so it sniffs as application/zip while its extension claims the concrete
// OOXML type. Tolerating only octet-stream as a generic sniff result rejected
// every office and OpenDocument upload.
//
// The set is iterated from [zipContainerExtensions] itself so a new entry
// cannot be added without being exercised; "archive.zip" is included separately
// as the case that already worked and must keep working.
func TestMIME_AllowsZipContainerDocuments(t *testing.T) {
	data := zipBytes(t)

	names := []string{"archive.zip"}
	for ext := range zipContainerExtensions {
		names = append(names, "document"+ext)
	}
	sort.Strings(names)

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			if err := runMIME(t, nil, name, data); err != nil {
				t.Errorf("should be allowed: %v", err)
			}
		})
	}
}

// TestMIME_ZipRejectionsNameTheirMechanism is the guard BUG-TDE1QO's review
// asked for. Two independent mechanisms reject ZIP bytes, and asserting only
// [ErrRejected] cannot tell them apart — so a .jar dropped from
// [deniedExtensions] still "passed", because on a developer box the leftover
// MIME-claim mismatch caught it. On a minimal image, where .jar resolves to no
// claim at all, that same change fails open silently.
//
// Asserting the message pins WHICH check fired, so the host-independent one
// cannot be quietly replaced by the host-dependent one.
func TestMIME_ZipRejectionsNameTheirMechanism(t *testing.T) {
	// deniedMsg is the extension deny set: consulted before any sniffing and
	// never consults the host MIME database.
	const deniedMsg = "is not allowed"
	// mismatchMsg is the sniff↔claim polyglot check, which does resolve a claim
	// through the host database.
	const mismatchMsg = "file extension implies"

	tests := []struct {
		name    string
		wantMsg string
	}{
		// Executable and installer containers — must be the deny set, since
		// most of these resolve to no claim on a minimal image.
		{"app.jar", deniedMsg},
		{"app.war", deniedMsg},
		{"app.apk", deniedMsg},
		{"addon.xpi", deniedMsg},
		{"ext.crx", deniedMsg},
		{"pkg.appx", deniedMsg},
		{"pkg.ipa", deniedMsg},
		// Macro-carrying office documents — likewise the deny set, not the
		// claim check that happens to catch them on a populated host.
		{"macro.docm", deniedMsg},
		{"macro.xlsm", deniedMsg},
		{"macro.pptm", deniedMsg},
		// Genuine polyglots: the extension claims a concrete non-ZIP format.
		// These are the cases the mismatch check exists for.
		{"fake.pdf", mismatchMsg},
		{"fake.png", mismatchMsg},
	}
	data := zipBytes(t)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := runMIME(t, nil, tc.name, data)
			if !errors.Is(err, ErrRejected) {
				t.Fatalf("zip bytes should be rejected, got %v", err)
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Errorf("rejected by the wrong check:\n got: %v\nwant message containing %q", err, tc.wantMsg)
			}
		})
	}
}

// TestMIME_ExecutableZipContainersDeniedByExtension pins the structural half of
// the rule above: every executable or macro-carrying container is in
// [deniedExtensions] rather than left to the claim check. See that test for why
// the distinction is load-bearing.
func TestMIME_ExecutableZipContainersDeniedByExtension(t *testing.T) {
	exts := []string{
		".jar", ".war", ".ear", ".apk", ".xpi", ".crx", ".appx", ".msix", ".ipa",
		".docm", ".xlsm", ".pptm", ".dotm", ".xltm", ".potm", ".xlam", ".ppam",
	}
	for _, ext := range exts {
		t.Run(ext, func(t *testing.T) {
			if !deniedExtensions[ext] {
				t.Errorf("%s must be in deniedExtensions, not left to the MIME-claim check", ext)
			}
		})
	}
}

// TestMIME_ExtensionSetsAreDisjoint pins that no extension is both tolerated as
// a ZIP container and denied outright. Deny currently wins by ordering inside
// [mimeProcessor.Process]; this makes the intent explicit rather than leaving a
// contradictory pair to be resolved by which check happens to run first.
func TestMIME_ExtensionSetsAreDisjoint(t *testing.T) {
	for ext := range zipContainerExtensions {
		if deniedExtensions[ext] {
			t.Errorf("%s is both a tolerated container and denied; pick one", ext)
		}
	}
}
