package attachment

import (
	"context"
	"errors"
	"io"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func policyMeta(prop metamodel.PropertyDef, global *metamodel.AttachmentsConfig) *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Attachments: global,
		Entities: map[string]metamodel.EntityDef{
			"doc": {Properties: map[string]metamodel.PropertyDef{"file": prop}},
		},
	}
}

func runPolicy(t *testing.T, m *metamodel.Metamodel, runner CommandRunner, data []byte) (string, error) {
	t.Helper()
	p := NewPolicyProcessor(m, runner)
	pc := ProcessContext{EntityID: "D1", EntityType: "doc", Property: "file", FileName: "a.png"}
	out, _, err := p.Process(context.Background(), pc, strings.NewReader(string(data)))
	if err != nil {
		return "", err
	}
	b, _ := io.ReadAll(out)
	return string(b), nil
}

func TestPolicy_MIMEOnlyWhenNoRunner(t *testing.T) {
	m := policyMeta(metamodel.PropertyDef{Type: metamodel.PropertyTypeFile}, nil)
	// png bytes, png name → allowed; passes through.
	out, err := runPolicy(t, m, nil, pngBytes)
	if err != nil {
		t.Fatalf("png should pass: %v", err)
	}
	if out != string(pngBytes) {
		t.Error("bytes should pass through unchanged")
	}
}

func TestPolicy_NoScanCommandSkipsScanning(t *testing.T) {
	// No scan_cmd anywhere → no scan; bytes pass through (with a runner present).
	m := policyMeta(metamodel.PropertyDef{Type: metamodel.PropertyTypeFile}, nil)
	r := newRunner(t)
	out, err := runPolicy(t, m, r, pngBytes)
	if err != nil {
		t.Fatalf("no scan command should not scan: %v", err)
	}
	if out != string(pngBytes) {
		t.Error("bytes should pass through unchanged")
	}
}

func TestPolicy_ScanCommandRejectsOnPositive(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fixtures")
	}
	// A configured scan command IS the intent to scan, fail-closed.
	m := policyMeta(
		metamodel.PropertyDef{
			Type:    metamodel.PropertyTypeFile,
			ScanCmd: []string{"false"}, // always "infected"
		},
		nil,
	)
	r := newRunner(t)
	_, err := runPolicy(t, m, r, pngBytes)
	if !errors.Is(err, ErrRejected) {
		t.Errorf("positive scan must reject; got %v", err)
	}
}

func TestPolicy_ScanCommandCleanPasses(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fixtures")
	}
	m := policyMeta(
		metamodel.PropertyDef{
			Type:    metamodel.PropertyTypeFile,
			ScanCmd: []string{"true"}, // always clean
		},
		nil,
	)
	r := newRunner(t)
	out, err := runPolicy(t, m, r, pngBytes)
	if err != nil {
		t.Fatalf("clean scan should pass: %v", err)
	}
	if out != string(pngBytes) {
		t.Error("clean scan must leave bytes unchanged")
	}
}

func TestPolicy_ScanOffSkipsGlobalCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fixtures")
	}
	// Global scan_cmd `false` would reject, but the property opts out → passes.
	m := policyMeta(
		metamodel.PropertyDef{Type: metamodel.PropertyTypeFile, Scan: metamodel.ScanOff},
		&metamodel.AttachmentsConfig{ScanCmd: []string{"false"}},
	)
	r := newRunner(t)
	out, err := runPolicy(t, m, r, pngBytes)
	if err != nil {
		t.Fatalf("scan: off must skip the global scanner: %v", err)
	}
	if out != string(pngBytes) {
		t.Error("opted-out property bytes should pass through")
	}
}

func TestPolicy_TransformRewritesBytes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fixtures")
	}
	m := policyMeta(
		metamodel.PropertyDef{
			Type: metamodel.PropertyTypeFile,
			// `cat {in}` echoes the input → identity, but proves the transform
			// path runs and replaces the stream.
			Transform: []metamodel.TransformStep{{Cmd: []string{"cat", templateIn}}},
		},
		nil,
	)
	r := newRunner(t)
	out, err := runPolicy(t, m, r, pngBytes)
	if err != nil {
		t.Fatalf("transform: %v", err)
	}
	if out != string(pngBytes) {
		t.Errorf("identity transform should preserve bytes; got %q", out)
	}
}

func TestPolicy_GlobalScanInheritedByProperty(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fixtures")
	}
	// Global scan_cmd `false`; property sets no command → inherits and rejects.
	m := policyMeta(
		metamodel.PropertyDef{Type: metamodel.PropertyTypeFile},
		&metamodel.AttachmentsConfig{ScanCmd: []string{"false"}},
	)
	r := newRunner(t)
	_, err := runPolicy(t, m, r, pngBytes)
	if !errors.Is(err, ErrRejected) {
		t.Errorf("property should inherit the global scan command; got %v", err)
	}
}

func TestPolicy_ScanRunsBeforeTransform(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fixtures")
	}
	// scan `false` rejects; the transform (`cat`) must never run — verified by
	// the rejection (a transform error would have a different message).
	m := policyMeta(
		metamodel.PropertyDef{
			Type:      metamodel.PropertyTypeFile,
			ScanCmd:   []string{"false"},
			Transform: []metamodel.TransformStep{{Cmd: []string{"cat"}}},
		},
		nil,
	)
	r, err := NewCmdRunner(2*time.Second, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	_, perr := runPolicy(t, m, r, pngBytes)
	if !errors.Is(perr, ErrRejected) || !strings.Contains(perr.Error(), "scan") {
		t.Errorf("scan must reject before transform; got %v", perr)
	}
}
