package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDownloadName(t *testing.T) {
	tests := []struct {
		name        string
		disposition string
		route       string
		want        string
	}{
		{"attachment name", `attachment; filename="DOC-1.pdf"`, "/api/v1/docs/DOC-1/_export?transform=pdf", "DOC-1.pdf"},
		{"path in name is dropped", `attachment; filename="../../etc/passwd"`, "/api/x", "passwd"},
		{"windows path in name", `attachment; filename="..\\evil\\a.txt"`, "/api/x", "a.txt"},
		{"control characters", "attachment; filename=\"a\x01b.txt\"", "/api/x", "ab.txt"},
		{"no header uses path", "", "/api/v1/_attachments/report.csv?x=1", "report.csv"},
		{"export endpoint has no name", "", "/api/v1/docs/_export", "download"},
		{"bad header falls back", "attachment; filename=", "/api/v1/files/a.txt", "a.txt"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := http.Header{}
			if tc.disposition != "" {
				h.Set("Content-Disposition", tc.disposition)
			}
			if got := downloadName(h, tc.route); got != tc.want {
				t.Fatalf("downloadName = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDownloads(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/docs/DOC-1/_export":
			w.Header().Set("Content-Disposition", `attachment; filename="DOC-1.md"`)
			_, _ = w.Write([]byte("# Doc"))
		default:
			http.Error(w, strings.Repeat("x", 400), http.StatusForbidden)
		}
	})
	dir := t.TempDir()
	var offered string
	dest := filepath.Join(dir, "saved.md")
	var alerts []string
	dl, err := newDownloads(Downloads{
		handler: handler,
		prompt: func(name string) (string, error) {
			offered = name
			return dest, nil
		},
		alert: func(_, msg string) { alerts = append(alerts, msg) },
	})
	require.NoError(t, err)

	require.Empty(t, dl.Save("/api/v1/docs/DOC-1/_export?transform=md"))
	assert.Equal(t, "DOC-1.md", offered)
	got, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, "# Doc", string(got))
	assert.Empty(t, alerts)

	msg := dl.Save("/api/v1/secret")
	assert.Contains(t, msg, "403")
	assert.Less(t, len(msg), 340, "a refusal's body is cut short")
	assert.Len(t, alerts, 1, "a failure is shown, since the page has no place for it")

	assert.NotEmpty(t, dl.Save("//evil.example/x"), "a protocol-relative path is refused")

	dest = ""
	require.Empty(t, dl.Save("/api/v1/docs/DOC-1/_export"), "closing the panel is a cancel, not an error")

	_, err = newDownloads(Downloads{handler: handler})
	require.Error(t, err)
}

// The cap stops a response while it is written, before it is all in memory.
func TestDownloads_CapStopsTheWriter(t *testing.T) {
	var refused bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		chunk := []byte(strings.Repeat("x", 10))
		for range 5 {
			if _, err := w.Write(chunk); err != nil {
				refused = true
				return
			}
		}
	})
	dl, err := newDownloads(Downloads{
		handler: handler,
		prompt:  func(string) (string, error) { return "", nil },
		alert:   func(string, string) {},
		limit:   25,
	})
	require.NoError(t, err)
	assert.Contains(t, dl.Save("/big"), "too large")
	assert.True(t, refused, "the handler sees the refusal")
}
