package attachment

import (
	"bytes"
	"context"
	"errors"
	"io"
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
