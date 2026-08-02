package attachment

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// realPNG / realJPEG produce actually-decodable image bytes (the shared
// pngBytes fixture is a header-only stub that sniffs as PNG but won't decode).
func realPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	m := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			m.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 100, A: 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, m); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func realJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	m := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			m.Set(x, y, color.RGBA{R: 200, G: uint8(x), B: uint8(y), A: 255})
		}
	}
	var b bytes.Buffer
	if err := jpeg.Encode(&b, m, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// runImagePolicy runs a property with the given transform through the policy
// processor (no command runner — native image steps need none) and returns the
// output bytes and the resulting file name.
func runImagePolicy(
	t *testing.T, prop metamodel.PropertyDef, fileName string, data []byte,
) (outBytes []byte, outName string, err error) {
	t.Helper()
	m := policyMeta(prop, nil)
	p := NewPolicyProcessor(m, nil)
	pc := ProcessContext{EntityID: "D1", EntityType: "doc", Property: "file", FileName: fileName}
	out, info, perr := p.Process(context.Background(), pc, bytes.NewReader(data))
	if perr != nil {
		return nil, "", perr
	}
	b, _ := io.ReadAll(out)
	return b, info.FileName, nil
}

func fileProp(steps ...metamodel.TransformStep) metamodel.PropertyDef {
	return metamodel.PropertyDef{Type: metamodel.PropertyTypeFile, Transform: steps}
}

func imageStep(reencode string) metamodel.TransformStep {
	return metamodel.TransformStep{Image: &metamodel.ImageStep{Reencode: reencode}}
}

// TestPolicy_NativeImage_RunsWithNilRunner is the core AC: an image: transform
// runs in-process with NO runner (out-of-box, no sandbox).
func TestPolicy_NativeImage_RunsWithNilRunner(t *testing.T) {
	prop := fileProp(imageStep("jpeg"))
	out, name, err := runImagePolicy(t, prop, "photo.png", realPNG(t, 16, 12))
	if err != nil {
		t.Fatalf("native image step should run without a runner: %v", err)
	}
	// Output must decode as JPEG.
	_, format, derr := image.DecodeConfig(bytes.NewReader(out))
	if derr != nil {
		t.Fatalf("output should decode: %v", derr)
	}
	if format != "jpeg" {
		t.Errorf("want jpeg output, got %q", format)
	}
	// Filename extension must be swapped to match (RR-7R3EYR).
	if !strings.HasSuffix(name, ".jpg") {
		t.Errorf("want .jpg extension, got %q", name)
	}
}

// TestPolicy_NativeImage_ReencodeToPNG checks the png target + extension swap.
func TestPolicy_NativeImage_ReencodeToPNG(t *testing.T) {
	prop := fileProp(imageStep("png"))
	out, name, err := runImagePolicy(t, prop, "scan.jpg", realJPEG(t, 10, 10))
	if err != nil {
		t.Fatalf("jpeg->png should run: %v", err)
	}
	_, format, _ := image.DecodeConfig(bytes.NewReader(out))
	if format != "png" {
		t.Errorf("want png output, got %q", format)
	}
	if !strings.HasSuffix(name, ".png") {
		t.Errorf("want .png extension, got %q", name)
	}
}

// TestPolicy_NativeImage_DefaultReencodeIsJPEG: an image step with no reencode
// defaults to jpeg.
func TestPolicy_NativeImage_DefaultReencodeIsJPEG(t *testing.T) {
	prop := fileProp(metamodel.TransformStep{Image: &metamodel.ImageStep{}})
	out, name, err := runImagePolicy(t, prop, "x.png", realPNG(t, 8, 8))
	if err != nil {
		t.Fatalf("default reencode should run: %v", err)
	}
	_, format, _ := image.DecodeConfig(bytes.NewReader(out))
	if format != "jpeg" {
		t.Errorf("default output should be jpeg, got %q", format)
	}
	if !strings.HasSuffix(name, ".jpg") {
		t.Errorf("want .jpg, got %q", name)
	}
}

// TestPolicy_NativeImage_UndecodableRejected: bytes that pass the MIME sniff as
// PNG but do not decode are rejected (422-class) by the native step.
func TestPolicy_NativeImage_UndecodableRejected(t *testing.T) {
	prop := fileProp(imageStep("jpeg"))
	// pngBytes is a header-only stub: sniffs as PNG, fails to decode.
	_, _, err := runImagePolicy(t, prop, "a.png", pngBytes)
	if !errors.Is(err, ErrRejected) {
		t.Errorf("undecodable image should be rejected: %v", err)
	}
}

// TestPolicy_NativeImage_StripsMetadata: a re-encode drops EXIF.
func TestPolicy_NativeImage_StripsMetadata(t *testing.T) {
	// Build a JPEG and confirm the native step output has no "Exif" marker.
	src := realJPEG(t, 12, 8)
	prop := fileProp(imageStep("jpeg"))
	out, _, err := runImagePolicy(t, prop, "p.jpg", src)
	if err != nil {
		t.Fatalf("reencode should run: %v", err)
	}
	if bytes.Contains(out, []byte("Exif")) {
		t.Error("re-encoded output should not carry EXIF")
	}
}

// TestPolicy_CmdStepWithNilRunnerRejected: a cmd: transform with no runner
// fails closed (unlike a native image step).
func TestPolicy_CmdStepWithNilRunnerRejected(t *testing.T) {
	prop := fileProp(metamodel.TransformStep{Cmd: []string{"vips", "thumbnail"}})
	_, _, err := runImagePolicy(t, prop, "a.png", realPNG(t, 8, 8))
	if !errors.Is(err, ErrRejected) {
		t.Errorf("cmd step without a runner should be rejected: %v", err)
	}
}

func TestSwapExt(t *testing.T) {
	tests := []struct {
		name, newExt, want string
	}{
		{"photo.png", "jpg", "photo.jpg"},
		{"a.b.webp", "png", "a.b.png"},
		{"noext", "jpg", "noext.jpg"},
		{"", "png", "image.png"},
		{"keep.png", "", "keep.png"},
	}
	for _, tc := range tests {
		if got := swapExt(tc.name, tc.newExt); got != tc.want {
			t.Errorf("swapExt(%q,%q)=%q want %q", tc.name, tc.newExt, got, tc.want)
		}
	}
}
