package realtime

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// filesRealtime is testRealtime's room with file properties: shot holds up
// to two files of at most 1 KiB, doc is a required single file, and
// evidence must carry a shot (its when: counts the files).
var filesRealtime = strings.Replace(testRealtime, `      typing: { persist: ephemeral, ttl: 5s, post: [member] }`,
	`      typing: { persist: ephemeral, ttl: 5s, post: [member] }
      photo:
        properties:
          caption: { type: string }
          shot: { type: file, max: 2, max_size: 1KiB }
        post: [member]
      form:
        properties:
          doc: { type: file, required: true }
        post: [member]
      evidence:
        properties:
          shot: { type: file }
        when: "message.shot >= 1"
        post: [member]
      mod-shot:
        properties:
          shot: { type: file }
        post: [mod]`, 1)

func photo(files ...FileInput) PostInput {
	return PostInput{Type: "photo", Props: map[string]any{"caption": "c"}, Files: files}
}

func shot(name, data string) FileInput {
	return FileInput{Property: "shot", Name: name, Data: []byte(data)}
}

func openFile(t *testing.T, h *harness, a *fakeAccess, seq int64, id string) (f channels.File, data string) {
	t.Helper()
	d, err := fileOpener(t, h.svc).OpenFile(as(a), room, seq, id)
	require.NoError(t, err)
	defer d.Body.Close()
	b, err := io.ReadAll(d.Body)
	require.NoError(t, err)
	return d.File, string(b)
}

func TestPost_Files(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withYAML(filesRealtime))
	bob := user("bob")
	m := h.post(t, bob, photo(shot("a.txt", "hello"), shot("a.txt", "again")))
	require.Len(t, m.Files, 2)
	require.Equal(t, "a.txt", m.Files[0].Name)
	require.Equal(t, "text/plain", m.Files[0].MIME)
	require.Equal(t, int64(5), m.Files[0].Size)
	require.Equal(t, "a-2.txt", m.Files[1].Name, "a repeated name gets a suffix")
	_, data := openFile(t, h, bob, m.Seq, m.Files[1].ID)
	require.Equal(t, "again", data)

	tr := h.post(t, bob, photo(shot("b", "upper:x")))
	require.Equal(t, "b.txt", tr.Files[0].Name, "a transform may rename")
	f, data := openFile(t, h, bob, tr.Seq, tr.Files[0].ID)
	require.Equal(t, tr.Files[0], f)
	require.Equal(t, "UPPER:X", data, "the transform's output is what is stored")

	got, err := h.svc.Read(as(bob), room, ReadRequest{})
	require.NoError(t, err)
	require.Equal(t, tr.Files, got.Messages[len(got.Messages)-1].Files, "a read renders the files")

	checks := h.files.checks
	require.Len(t, checks, 3)
	require.Equal(t, roomKey(), checks[0].Channel)
	require.Equal(t, "shot", checks[0].Property)
}

func TestPost_FilesRefused(t *testing.T) {
	t.Parallel()
	big := strings.Repeat("x", 1025)
	tests := []struct {
		name   string
		in     PostInput
		status int
		typ    string
		checks int
	}{
		{"too many", photo(shot("1", "a"), shot("2", "b"), shot("3", "c")), http.StatusBadRequest, TypeInvalid, 0},
		{"over max_size", photo(shot("1", big)), http.StatusRequestEntityTooLarge, TypeTooLarge, 0},
		{"not a file property", photo(FileInput{Property: "caption", Name: "a", Data: []byte("a")}),
			http.StatusBadRequest, TypeInvalid, 0},
		{"type without files", PostInput{Type: "text", Props: map[string]any{"body": "b"},
			Files: []FileInput{shot("a", "a")}}, http.StatusBadRequest, TypeInvalid, 0},
		{"required missing", PostInput{Type: "form"}, http.StatusBadRequest, TypeInvalid, 0},
		{"when counts files", PostInput{Type: "evidence"}, http.StatusForbidden, TypePostDenied, 0},
		{"value for a file property", PostInput{Type: "photo", Props: map[string]any{"shot": "x"}},
			http.StatusBadRequest, TypeInvalid, 0},
		{"policy refuses", photo(shot("ok", "fine"), shot("bad", "VIRUS")), http.StatusUnprocessableEntity,
			TypeFileRefused, 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t, withYAML(filesRealtime))
			_, err := h.svc.Post(as(user("bob")), room, tc.in)
			requireDenial(t, err, tc.status, tc.typ)
			require.Equal(t, tc.checks, h.files.count(), "the policy runs only after the cheap checks")
			_, err = h.store.Head(t.Context(), roomKey())
			require.ErrorIs(t, err, channels.ErrNotFound, "nothing is stored")
		})
	}
	h := newHarness(t, withYAML(filesRealtime))
	m := h.post(t, user("bob"), PostInput{Type: "evidence", Files: []FileInput{shot("a", "a")}})
	require.Len(t, m.Files, 1, "when: holds with a file")
}

func TestOpenFile_NotFound(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withYAML(filesRealtime))
	bob := user("bob")
	m := h.post(t, bob, photo(shot("a.txt", "hello")))
	plain := h.post(t, bob, text("plain"))
	id := m.Files[0].ID

	for name, tc := range map[string]struct {
		seq int64
		id  string
	}{
		"bad id":        {m.Seq, "../x"},
		"unknown id":    {m.Seq, strings.Repeat("a", 26)},
		"other message": {plain.Seq, id},
		"no message":    {99, id},
	} {
		_, err := fileOpener(t, h.svc).OpenFile(as(bob), room, tc.seq, tc.id)
		requireDenial(t, err, http.StatusNotFound, TypeFileNotFound)
		_ = name
	}

	// A caller who cannot read the channel gets the read denial first.
	_, err := fileOpener(t, h.svc).OpenFile(as(user("mallory", "none")), RawAddress{EntityType: "ticket", Anchor: "TKT-404",
		Type: "room"}, m.Seq, id)
	require.Error(t, err)

	_, err = h.svc.Delete(as(bob), room, m.Seq, "")
	require.NoError(t, err)
	_, err = fileOpener(t, h.svc).OpenFile(as(bob), room, m.Seq, id)
	requireDenial(t, err, http.StatusNotFound, TypeFileNotFound)
}

func TestEdit_KeepsFiles(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withYAML(filesRealtime))
	bob := user("bob")
	m := h.post(t, bob, PostInput{Type: "evidence", Files: []FileInput{shot("a", "kept")}})
	got, err := h.svc.Edit(as(bob), room, m.Seq, map[string]any{})
	require.NoError(t, err, "when: sees the stored files on edit")
	require.Equal(t, m.Files, got.Files)
	_, data := openFile(t, h, bob, m.Seq, m.Files[0].ID)
	require.Equal(t, "kept", data)
}

func TestFileRate(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withYAML(filesRealtime))
	bob := user("bob")
	for range 10 {
		h.post(t, bob, photo(shot("a", "a")))
	}
	_, err := h.svc.Post(as(bob), room, photo(shot("a", "a")))
	requireDenial(t, err, http.StatusTooManyRequests, TypeRateLimited)
	h.post(t, bob, photo())
}

// fileOpener is the file reader over svc.
func fileOpener(t *testing.T, svc *Service) *FileOpener {
	t.Helper()
	o, err := NewFileOpener(svc)
	if err != nil {
		t.Fatal(err)
	}
	return o
}

func TestNewFileOpener_RejectsNil(t *testing.T) {
	if _, err := NewFileOpener(nil); err == nil {
		t.Fatal("NewFileOpener(nil) = nil error")
	}
}

// TestFileLimitsAgree pins the store's file limits to the ones realtime.yaml
// is checked against, which are declared twice to keep the imports apart.
func TestFileLimitsAgree(t *testing.T) {
	for _, tc := range []struct {
		name          string
		store, config int
	}{
		{"files per message", channels.MaxFilesPerMessage, realtimelimits.MaxFilesPerMessage},
		{"file bytes", channels.MaxFileBytes, realtimelimits.MaxFileBytes},
		{"message file bytes", channels.MaxMessageFileBytes, realtimelimits.MaxMessageFileBytes},
	} {
		if tc.store != tc.config {
			t.Errorf("%s: channels has %d, realtimelimits %d", tc.name, tc.store, tc.config)
		}
	}
}

func TestCheckUpload(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withYAML(filesRealtime))
	o := fileOpener(t, h.svc)
	require.NoError(t, o.CheckUpload(as(user("bob")), room, "photo"))
	require.NoError(t, o.CheckUpload(as(user("mo", "chat.moderate")), room, "mod-shot"))
	for _, tc := range []struct {
		name, typ string
		who       *fakeAccess
		status    int
		denial    string
	}{
		{"unknown type", "nope", user("bob"), http.StatusBadRequest, TypeInvalid},
		{"type without files", "text", user("bob"), http.StatusBadRequest, TypeInvalid},
		{"may not post", "mod-shot", user("bob"), http.StatusForbidden, TypePostDenied},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			requireDenial(t, o.CheckUpload(as(tc.who), room, tc.typ), tc.status, tc.denial)
		})
	}
	err := o.CheckUpload(as(user("mallory", "none")), RawAddress{EntityType: "ticket", Anchor: "TKT-404",
		Type: "room"}, "photo")
	require.Error(t, err, "a caller who cannot read the channel is refused")
}

func TestOpenFile_RateLimited(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withYAML(filesRealtime))
	bob := user("bob")
	m := h.post(t, bob, photo(shot("a.txt", "hello")))
	o := fileOpener(t, h.svc)
	for range realtimelimits.FileReadRate.Count {
		d, err := o.OpenFile(as(bob), room, m.Seq, m.Files[0].ID)
		require.NoError(t, err)
		d.Body.Close()
	}
	_, err := o.OpenFile(as(bob), room, m.Seq, m.Files[0].ID)
	requireDenial(t, err, http.StatusTooManyRequests, TypeRateLimited)
}
