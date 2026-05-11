package dataentry

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"errors"
	"strings"
	"testing"
)

// minimalManifestYAML returns a valid theme.yaml body with optional
// overrides. Tests only set the fields they care about.
func minimalManifestYAML(extra map[string]string) string {
	defaults := map[string]string{
		"name":    "Test Theme",
		"version": "1.0.0",
		"accent":  "#6366f1",
	}
	for k, v := range extra {
		defaults[k] = v
	}
	var b strings.Builder
	for k, v := range defaults {
		b.WriteString(k)
		b.WriteString(`: "`)
		b.WriteString(v)
		b.WriteString("\"\n")
	}
	return b.String()
}

// buildZip writes a zip archive from a name → bytes map. Iteration
// order is unspecified; parseThemePackage indexes by name so order
// does not matter.
func buildZip(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, body := range entries {
		f, err := w.Create(name)
		if err != nil {
			t.Fatalf("zip.Create(%q): %v", name, err)
		}
		if _, err := f.Write(body); err != nil {
			t.Fatalf("zip write(%q): %v", name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

func TestParseThemePackage_PaletteOnly(t *testing.T) {
	z := buildZip(t, map[string][]byte{
		"theme.yaml": []byte(minimalManifestYAML(nil)),
	})
	pkg, err := parseThemePackage(z)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if pkg.Manifest == nil {
		t.Fatal("manifest is nil")
	}
	if pkg.Manifest.Name != "Test Theme" {
		t.Errorf("name = %q", pkg.Manifest.Name)
	}
	if pkg.Logo != nil {
		t.Errorf("expected no logo, got %+v", pkg.Logo)
	}
}

func TestParseThemePackage_WithLogo(t *testing.T) {
	pngBytes := makeTinyPNG(t)
	z := buildZip(t, map[string][]byte{
		"theme.yaml": []byte(minimalManifestYAML(map[string]string{"logo": "logo.png"})),
		"logo.png":   pngBytes,
	})
	pkg, err := parseThemePackage(z)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if pkg.Logo == nil {
		t.Fatal("expected logo, got nil")
	}
	if pkg.Logo.Ext != "png" {
		t.Errorf("ext = %q", pkg.Logo.Ext)
	}
	if !bytes.Equal(pkg.Logo.Bytes, pngBytes) {
		t.Error("logo bytes do not match")
	}
}

func TestParseThemePackage_TrustsSniffOverManifestExtension(t *testing.T) {
	// Manifest claims `.png` but the bytes are SVG; we trust the
	// sniff and store as svg.
	z := buildZip(t, map[string][]byte{
		"theme.yaml": []byte(minimalManifestYAML(map[string]string{"logo": "logo.png"})),
		"logo.png":   hostileSVGBytes(),
	})
	pkg, err := parseThemePackage(z)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if pkg.Logo == nil || pkg.Logo.Ext != "svg" {
		t.Errorf("expected sniffed ext=svg, got %+v", pkg.Logo)
	}
}

func TestParseThemePackage_IgnoresExtraEntries(t *testing.T) {
	z := buildZip(t, map[string][]byte{
		"theme.yaml": []byte(minimalManifestYAML(nil)),
		"README.md":  []byte("# notes"),
		".DS_Store":  {0xff, 0xfe},
	})
	if _, err := parseThemePackage(z); err != nil {
		t.Errorf("expected extras to be ignored, got %v", err)
	}
}

func TestParseThemePackage_Rejects(t *testing.T) {
	pngBytes := makeTinyPNG(t)
	tests := []struct {
		name    string
		zip     map[string][]byte
		raw     []byte
		wantErr error
	}{
		{
			name:    "missing manifest",
			zip:     map[string][]byte{"logo.png": pngBytes},
			wantErr: errMissingManifest,
		},
		{
			name: "manifest name empty",
			zip: map[string][]byte{
				"theme.yaml": []byte(minimalManifestYAML(map[string]string{"name": ""})),
			},
			wantErr: errInvalidManifest,
		},
		{
			name: "manifest references missing logo",
			zip: map[string][]byte{
				"theme.yaml": []byte(minimalManifestYAML(map[string]string{"logo": "logo.png"})),
			},
			wantErr: errMissingLogo,
		},
		{
			name: "logo bytes are not an image",
			zip: map[string][]byte{
				"theme.yaml": []byte(minimalManifestYAML(map[string]string{"logo": "logo.png"})),
				"logo.png":   []byte("hello world"),
			},
			// Surfaces as "logo: unsupported format ..."; non-sentinel.
		},
		{
			name: "path traversal in entry name",
			zip: map[string][]byte{
				"theme.yaml":   []byte(minimalManifestYAML(nil)),
				"../etc/hosts": []byte("evil"),
			},
			wantErr: errZipPathTraversal,
		},
		{
			name: "subdirectory in entry name",
			zip: map[string][]byte{
				"theme.yaml":  []byte(minimalManifestYAML(nil)),
				"sub/foo.txt": []byte("nope"),
			},
			wantErr: errZipPathTraversal,
		},
		{
			name:    "not a zip",
			raw:     []byte("just some bytes"),
			wantErr: errNotAZip,
		},
		{
			name: "malformed manifest yaml",
			zip: map[string][]byte{
				"theme.yaml": []byte("not: valid: yaml: ::"),
			},
			wantErr: errInvalidManifest,
		},
		{
			name: "bad palette color",
			zip: map[string][]byte{
				"theme.yaml": []byte(minimalManifestYAML(map[string]string{"accent": "not-a-color"})),
			},
			wantErr: errInvalidManifest,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := tt.raw
			if raw == nil {
				raw = buildZip(t, tt.zip)
			}
			_, err := parseThemePackage(raw)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestParseThemePackage_LogoTooLarge(t *testing.T) {
	// Use cryptographically random padding so DEFLATE can't compress
	// it; otherwise the zip-bomb expansion-ratio guard fires before
	// the logo-size guard. This test is about the per-asset cap.
	tooBig := make([]byte, MaxUserLogoBytes+1)
	pngHeader := makeTinyPNG(t)
	copy(tooBig, pngHeader)
	if _, err := rand.Read(tooBig[len(pngHeader):]); err != nil {
		t.Fatalf("rand: %v", err)
	}
	z := buildZip(t, map[string][]byte{
		"theme.yaml": []byte(minimalManifestYAML(map[string]string{"logo": "logo.png"})),
		"logo.png":   tooBig,
	})
	_, err := parseThemePackage(z)
	if !errors.Is(err, errLogoTooLarge) {
		t.Errorf("expected errLogoTooLarge, got %v", err)
	}
}

func TestParseThemePackage_ZipBomb(t *testing.T) {
	// A highly compressible entry the zip-bomb guard should reject.
	hugePayload := make([]byte, ThemePackageMaxBytes+1)
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	mf, _ := w.Create("theme.yaml")
	_, _ = mf.Write([]byte(minimalManifestYAML(nil)))
	hf, _ := w.Create("logo.png")
	_, _ = hf.Write(hugePayload)
	if err := w.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	_, err := parseThemePackage(buf.Bytes())
	if !errors.Is(err, errZipUncompressed) && !errors.Is(err, errZipBomb) {
		t.Errorf("expected errZipUncompressed or errZipBomb, got %v", err)
	}
}

func TestParseThemePackage_RejectsDuplicateEntries(t *testing.T) {
	// Hand-roll a zip with two entries named "theme.yaml" — buildZip's
	// map can't represent duplicates by definition.
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for range 2 {
		f, err := w.Create("theme.yaml")
		if err != nil {
			t.Fatalf("zip.Create: %v", err)
		}
		if _, err := f.Write([]byte(minimalManifestYAML(nil))); err != nil {
			t.Fatalf("zip write: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	_, err := parseThemePackage(buf.Bytes())
	if !errors.Is(err, errDuplicateEntry) {
		t.Errorf("expected errDuplicateEntry, got %v", err)
	}
}

func TestParseThemePackage_TotalTooLarge(t *testing.T) {
	// Raw input larger than ThemePackageMaxBytes is rejected before
	// zip parsing.
	raw := make([]byte, ThemePackageMaxBytes+1)
	_, err := parseThemePackage(raw)
	if !errors.Is(err, errZipUncompressed) {
		t.Errorf("expected errZipUncompressed, got %v", err)
	}
}
