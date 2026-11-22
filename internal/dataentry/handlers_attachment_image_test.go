package dataentry

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// TestImageProperty_Upload pins the server-side checks on a
// `display: {image: true}` property (TKT-WG1410D): content sniffing, the size
// cap, the refusal of SVG, and the hardened download.
func TestImageProperty_Upload(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	d := writeACL(t, app)
	setTestACL(t, app, d)

	t.Run("accepts a png and serves it hardened", func(t *testing.T) {
		rec := putAttachmentAs(aliceCtx(), t, app, d, "TKT-001", "logo", "logo.png", pngBytes("pixels"))
		if rec.Code != http.StatusOK {
			t.Fatalf("upload: got %d, want 200; body=%s", rec.Code, rec.Body)
		}
		get := getAttachmentAs(aliceCtx(), t, app, d, "ticket", "tickets", "TKT-001", "logo", "logo.png")
		if get.Code != http.StatusOK {
			t.Fatalf("GET: got %d", get.Code)
		}
		h := get.Header()
		hardened := h.Get("Content-Type") == "image/png" &&
			h.Get("X-Content-Type-Options") == "nosniff" &&
			strings.Contains(h.Get("Content-Security-Policy"), "sandbox") &&
			strings.HasPrefix(h.Get("Content-Disposition"), "attachment")
		if !hardened {
			t.Errorf("download headers not hardened: %v", h)
		}
	})

	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)
	refused := []struct {
		name string
		file string
		data []byte
	}{
		{"svg", "logo.svg", svg},
		{"svg named png", "logo.png", svg},
		{"html named png", "logo.png", []byte("<html><script>alert(1)</script></html>")},
		{"text", "logo.txt", []byte("not an image")},
		{"pdf", "logo.pdf", []byte("%PDF-1.4 ...")},
		{"unknown binary named png", "logo.png", bytes.Repeat([]byte{0x01}, 64)},
	}
	for _, tc := range refused {
		t.Run("refuses "+tc.name, func(t *testing.T) {
			rec := putAttachmentAs(aliceCtx(), t, app, d, "TKT-001", "logo", tc.file, tc.data)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("got %d, want 422; body=%s", rec.Code, rec.Body)
			}
		})
	}

	t.Run("refuses a file over the image cap", func(t *testing.T) {
		big := append(pngBytes("pixels"), make([]byte, metamodel.ImageMaxBytes)...)
		rec := putAttachmentAs(aliceCtx(), t, app, d, "TKT-001", "logo", "big.png", big)
		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("got %d, want 413; body=%s", rec.Code, rec.Body)
		}
	})
}
