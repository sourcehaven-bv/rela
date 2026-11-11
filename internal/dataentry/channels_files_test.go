package dataentry

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"testing"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// filesRealtime is channelTestRealtime with a shots channel whose photo
// message holds up to two PNG images, and whose note holds any file.
const filesRealtime = channelTestRealtime + `  shots:
    attach_to: [ticket]
    read: { from: attached }
    roles:
      member: { from: attached }
    message_types:
      photo:
        properties:
          caption: { type: string }
          image:   { type: file, max: 2, accept: [image/png] }
        post: [member]
      note:
        properties:
          body: { type: string }
          doc:  { type: file }
        post: [member]
    message_ops:
      delete_own: { roles: [member] }
`

const shotsPath = "/api/v1/tickets/TKT-1/_channels/shots"

// pngBytes is a PNG header followed by payload, which sniffs as image/png.
func pngBytes(payload string) []byte {
	return append([]byte("\x89PNG\r\n\x1a\n"), payload...)
}

// mpPart is one part of a multipart post.
type mpPart struct {
	name, file string
	data       []byte
}

func channelMultipart(t *testing.T, parts ...mpPart) (body []byte, contentType string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for _, p := range parts {
		h := textproto.MIMEHeader{}
		disp := `form-data; name="` + p.name + `"`
		if p.file != "" {
			disp += `; filename="` + p.file + `"`
		}
		h.Set("Content-Disposition", disp)
		w, err := mw.CreatePart(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(p.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes(), mw.FormDataContentType()
}

// rawResponse is the status and headers of a response whose body doRaw
// has read and closed.
type rawResponse struct {
	StatusCode int
	Header     http.Header
}

// doRaw sends body with contentType as user and returns the response.
func (e *channelEnv) doRaw(t *testing.T, user, method, path, contentType string, body []byte) (got rawResponse, data []byte) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), method, e.srv.URL+path, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set(testUserHeader, user)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := e.srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return rawResponse{StatusCode: resp.StatusCode, Header: resp.Header}, b
}

func newFilesEnv(t *testing.T) *channelEnv {
	t.Helper()
	env := newChannelEnv(t)
	env.ch.cfg.Store(env.ch.parse(t, filesRealtime))
	return env
}

// AC2/AC3: a member posts a PNG with a message; a reader sees the entry
// and downloads the same bytes as a hardened attachment.
func TestChannelFiles_PostAndDownload(t *testing.T) {
	env := newFilesEnv(t)
	img := pngBytes("pixels-9f2")
	body, ct := channelMultipart(t,
		mpPart{name: "message", data: []byte(`{"type":"photo","props":{"caption":"look"}}`)},
		mpPart{name: "file:image", file: "shot.png", data: img},
	)
	resp, b := env.doRaw(t, "alice", http.MethodPost, shotsPath+"/messages", ct, body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("post = %d %s, want 201", resp.StatusCode, b)
	}
	msg := decodeAs[v1.Message](t, b)
	if len(msg.Files) != 1 {
		t.Fatalf("files = %+v, want one", msg.Files)
	}
	f := msg.Files[0]
	if f.Property != "image" || f.Name != "shot.png" || f.MIME != "image/png" || f.Size != int64(len(img)) {
		t.Fatalf("file = %+v", f)
	}

	// A plain page carries the entry too.
	resp, b = env.doRaw(t, "bob", http.MethodGet, shotsPath+"/messages", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("page = %d %s", resp.StatusCode, b)
	}
	page := decodeAs[v1.MessagePage](t, b)
	if len(page.Messages) != 1 || len(page.Messages[0].Files) != 1 || page.Messages[0].Files[0] != f {
		t.Fatalf("page files = %+v", page.Messages)
	}

	path := shotsPath + "/messages/" + strconv.FormatInt(msg.Seq, 10) + "/files/" + f.ID
	resp, b = env.doRaw(t, "bob", http.MethodGet, path, "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("download = %d %s", resp.StatusCode, b)
	}
	if !bytes.Equal(b, img) {
		t.Fatalf("downloaded %q, want %q", b, img)
	}
	for k, want := range map[string]string{
		"Content-Type":            "image/png",
		"X-Content-Type-Options":  "nosniff",
		"Content-Security-Policy": "sandbox; default-src 'none'",
		"Content-Disposition":     `attachment; filename="shot.png"`,
		"Cache-Control":           "private, no-store",
	} {
		if got := resp.Header.Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}

	// Only POST/GET on the right routes.
	resp, _ = env.doRaw(t, "bob", http.MethodPost, path, "", nil)
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("POST file = %d, want 405", resp.StatusCode)
	}
}

// AC4/AC5: a download the caller may not make, of a file that is not
// there, or of a removed message is a 404 that tells none apart.
func TestChannelFiles_DownloadNotFound(t *testing.T) {
	env := newFilesEnv(t)
	body, ct := channelMultipart(t,
		mpPart{name: "message", data: []byte(`{"type":"note","props":{"body":"x"}}`)},
		mpPart{name: "file:doc", file: "a.txt", data: []byte("secret-doc")},
	)
	resp, b := env.doRaw(t, "alice", http.MethodPost, shotsPath+"/messages", ct, body)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("post = %d %s", resp.StatusCode, b)
	}
	msg := decodeAs[v1.Message](t, b)
	id := msg.Files[0].ID
	seq := strconv.FormatInt(msg.Seq, 10)
	for _, tc := range []struct {
		name, user, path string
		code             int
	}{
		{"wrong id", "bob", shotsPath + "/messages/" + seq + "/files/" + strings.Repeat("a", len(id)), http.StatusNotFound},
		{"bad id", "bob", shotsPath + "/messages/" + seq + "/files/a.b", http.StatusNotFound},
		{"wrong seq", "bob", shotsPath + "/messages/99/files/" + id, http.StatusNotFound},
		{"other channel", "bob", "/api/v1/tickets/TKT-2/_channels/shots/messages/" + seq + "/files/" + id,
			http.StatusNotFound},
		{"hidden anchor", "bob", "/api/v1/features/FEAT-1/_channels/shots/messages/" + seq + "/files/" + id,
			http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, data := env.doRaw(t, tc.user, http.MethodGet, tc.path, "", nil)
			if got.StatusCode != tc.code {
				t.Fatalf("GET %s = %d %s, want %d", tc.path, got.StatusCode, data, tc.code)
			}
			if bytes.Contains(data, []byte("secret-doc")) {
				t.Fatal("a refused download carries the bytes")
			}
		})
	}

	// Removing the message removes its file.
	resp, b = env.doRaw(t, "alice", http.MethodDelete, shotsPath+"/messages/"+seq, "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("delete = %d %s", resp.StatusCode, b)
	}
	resp, b = env.doRaw(t, "bob", http.MethodGet, shotsPath+"/messages/"+seq+"/files/"+id, "", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("download after delete = %d %s, want 404", resp.StatusCode, b)
	}
}

// AC6/AC7: malformed bodies are 400, oversize ones 413, refused content
// 422; nothing is posted.
func TestChannelFiles_PostRefused(t *testing.T) {
	env := newFilesEnv(t)
	photo := mpPart{name: "message", data: []byte(`{"type":"photo"}`)}
	png := func(name string) mpPart { return mpPart{name: "file:image", file: name, data: pngBytes("p")} }
	for _, tc := range []struct {
		name  string
		parts []mpPart
		code  int
		ptype string
	}{
		{"no message part", []mpPart{png("a.png")}, http.StatusBadRequest, "invalid_request"},
		{"message after file", []mpPart{png("a.png"), photo}, http.StatusBadRequest, "invalid_request"},
		{"empty body", nil, http.StatusBadRequest, "invalid_request"},
		{"unknown part", []mpPart{photo, {name: "extra", data: []byte("x")}}, http.StatusBadRequest, "invalid_request"},
		{"file part without name", []mpPart{photo, {name: "file:image", data: pngBytes("p")}},
			http.StatusBadRequest, "invalid_request"},
		{"bad json", []mpPart{{name: "message", data: []byte(`{"type":`)}}, http.StatusBadRequest, "invalid_request"},
		{"unknown field", []mpPart{{name: "message", data: []byte(`{"type":"photo","x":1}`)}},
			http.StatusBadRequest, "invalid_request"},
		{"too many parts", []mpPart{photo, png("1.png"), png("2.png"), png("3.png"), png("4.png"), png("5.png")},
			http.StatusBadRequest, "invalid_request"},
		{"over max", []mpPart{photo, png("1.png"), png("2.png"), png("3.png")},
			http.StatusBadRequest, realtime.TypeInvalid},
		{"undeclared property", []mpPart{photo, {name: "file:caption", file: "a.png", data: pngBytes("p")}},
			http.StatusBadRequest, realtime.TypeInvalid},
		{"not accepted", []mpPart{photo, {name: "file:image", file: "a.txt", data: []byte("plain text")}},
			http.StatusUnprocessableEntity, realtime.TypeFileRefused},
		{"over the message cap", []mpPart{{name: "message", data: []byte(`{"type":"note"}`)},
			{name: "file:doc", file: "big.bin", data: make([]byte, realtimelimits.MaxMessageFileBytes+1)}},
			http.StatusRequestEntityTooLarge, realtime.TypeTooLarge},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, ct := channelMultipart(t, tc.parts...)
			resp, b := env.doRaw(t, "alice", http.MethodPost, shotsPath+"/messages", ct, body)
			if resp.StatusCode != tc.code || problemType(t, b) != tc.ptype {
				t.Fatalf("post = %d %s, want %d %s", resp.StatusCode, b, tc.code, tc.ptype)
			}
		})
	}
	resp, b := env.doRaw(t, "alice", http.MethodGet, shotsPath+"/messages", "", nil)
	if resp.StatusCode == http.StatusOK && len(decodeAs[v1.MessagePage](t, b).Messages) != 0 {
		t.Fatalf("a refused post stored a message: %s", b)
	}
}

// A post while every upload slot is taken is a 503 with Retry-After, before
// the body is read.
func TestChannelFiles_BusyUploads(t *testing.T) {
	env := newFilesEnv(t)
	var releases []func()
	for {
		release, ok := env.app.attachmentUploads.TryAcquire()
		if !ok {
			break
		}
		releases = append(releases, release)
	}
	defer func() {
		for _, r := range releases {
			r()
		}
	}()
	body, ct := channelMultipart(t, mpPart{name: "message", data: []byte(`{"type":"note"}`)})
	resp, b := env.doRaw(t, "alice", http.MethodPost, shotsPath+"/messages", ct, body)
	if resp.StatusCode != http.StatusServiceUnavailable || resp.Header.Get("Retry-After") == "" {
		t.Fatalf("post = %d %s, want 503 with Retry-After", resp.StatusCode, b)
	}
	// The preflight answers before a slot is sought, so a post that would
	// be refused anyway is never told to retry.
	for _, tc := range []struct{ name, user, msg string }{
		{"unknown type", "alice", `{"type":"nope"}`},
		{"cannot read", "nobody", `{"type":"note"}`},
	} {
		body, ct := channelMultipart(t, mpPart{name: "message", data: []byte(tc.msg)})
		got, data := env.doRaw(t, tc.user, http.MethodPost, shotsPath+"/messages", ct, body)
		if got.StatusCode == http.StatusServiceUnavailable || got.StatusCode < 400 {
			t.Fatalf("%s: post = %d %s, want the preflight refusal", tc.name, got.StatusCode, data)
		}
	}
	// A JSON post takes no slot.
	resp, b = env.doRaw(t, "alice", http.MethodPost, shotsPath+"/messages", "application/json",
		[]byte(`{"type":"note","props":{"body":"x"}}`))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("json post = %d %s, want 201", resp.StatusCode, b)
	}
}
