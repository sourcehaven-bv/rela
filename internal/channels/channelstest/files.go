package channelstest

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
)

var filesTests = []conformanceTest{
	{"FilesAppendAndOpen", testFilesAppendOpen},
	{"FilesCaps", testFilesCaps},
	{"FilesEditKeeps", testFilesEditKeeps},
	{"FilesRemoveErases", testFilesRemoveErases},
	{"FilesDeleteErases", testFilesDeleteErases},
	{"FilesFollowRekey", testFilesRekey},
	{"FilesTombstoneAndPurge", testFilesTombstonePurge},
}

// withFiles is a text message carrying files, each the property "shot".
func withFiles(body string, files ...channels.NewFile) channels.NewMessage {
	m := text(body)
	m.Files = files
	return m
}

func newFile(name, data string) channels.NewFile {
	return channels.NewFile{Property: "shot", Name: name, MIME: "text/plain", Data: []byte(data)}
}

// readFile opens file id of message seq in k and returns its entry and
// bytes.
func readFile(t *testing.T, s channels.Store, k channels.Key, seq int64, id string) (f channels.File, data string) {
	t.Helper()
	f, rc, err := s.OpenFile(context.Background(), k, seq, id)
	require.NoError(t, err)
	defer rc.Close()
	b, err := io.ReadAll(rc)
	require.NoError(t, err)
	return f, string(b)
}

func requireNoFile(t *testing.T, s channels.Store, k channels.Key, seq int64, id string) {
	t.Helper()
	_, rc, err := s.OpenFile(context.Background(), k, seq, id)
	require.ErrorIs(t, err, channels.ErrNotFound)
	require.Nil(t, rc)
}

func testFilesAppendOpen(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	mustAppend(t, h.Store, k, text("plain"))
	got := mustAppend(t, h.Store, k, withFiles("two files", newFile("a.txt", "alpha-bytes"), newFile("b.txt", "beta")))
	require.Len(t, got.Files, 2)
	for i, want := range []channels.File{
		{Property: "shot", Name: "a.txt", MIME: "text/plain", Size: 11},
		{Property: "shot", Name: "b.txt", MIME: "text/plain", Size: 4},
	} {
		require.True(t, channels.ValidFileID(got.Files[i].ID), got.Files[i].ID)
		want.ID = got.Files[i].ID
		require.Equal(t, want, got.Files[i])
	}
	require.NotEqual(t, got.Files[0].ID, got.Files[1].ID)

	stored, err := h.Store.Get(ctx, k, got.Seq)
	require.NoError(t, err)
	require.Equal(t, got.Files, stored.Files, "the entries are stored with the message")
	page, err := h.Store.Messages(ctx, k, channels.Page{})
	require.NoError(t, err)
	require.Equal(t, got.Files, page[1].Files)
	require.Empty(t, page[0].Files)

	f, data := readFile(t, h.Store, k, got.Seq, got.Files[0].ID)
	require.Equal(t, got.Files[0], f)
	require.Equal(t, "alpha-bytes", data)
	_, data = readFile(t, h.Store, k, got.Seq, got.Files[1].ID)
	require.Equal(t, "beta", data)

	// Only an id the message lists opens: not another message's, not a
	// made-up one, not one in another channel.
	requireNoFile(t, h.Store, k, 1, got.Files[0].ID)
	requireNoFile(t, h.Store, k, got.Seq, strings.Repeat("a", 26))
	requireNoFile(t, h.Store, k, got.Seq, "../meta.yaml")
	requireNoFile(t, h.Store, key("TKT-1", "other"), got.Seq, got.Files[0].ID)
	requireNoFile(t, h.Store, k, 99, got.Files[0].ID)
}

func testFilesCaps(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	big := channels.NewFile{Property: "shot", Name: "big.bin", MIME: "application/octet-stream",
		Data: make([]byte, channels.MaxFileBytes)}
	tests := []struct {
		name  string
		files []channels.NewFile
		want  error
	}{
		{"too many", []channels.NewFile{newFile("1", "x"), newFile("2", "x"), newFile("3", "x"),
			newFile("4", "x"), newFile("5", "x")}, channels.ErrTooLarge},
		{"one too big", []channels.NewFile{{Property: "shot", Name: "b", MIME: "x/y",
			Data: make([]byte, channels.MaxFileBytes+1)}}, channels.ErrTooLarge},
		{"together too big", []channels.NewFile{big, {Property: "shot", Name: "c", MIME: "x/y",
			Data: make([]byte, channels.MaxMessageFileBytes-channels.MaxFileBytes+1)}}, channels.ErrTooLarge},
		{"empty", []channels.NewFile{newFile("e", "")}, channels.ErrInvalidMessage},
		{"same name", []channels.NewFile{newFile("a", "1"), newFile("a", "2")}, channels.ErrInvalidMessage},
		{"slash in name", []channels.NewFile{newFile("a/b", "1")}, channels.ErrInvalidMessage},
		{"dot dot", []channels.NewFile{newFile("..", "1")}, channels.ErrInvalidMessage},
		{"control in name", []channels.NewFile{newFile("a\nb", "1")}, channels.ErrInvalidMessage},
		{"bad property", []channels.NewFile{{Property: "Shot", Name: "a", MIME: "x/y", Data: []byte("1")}},
			channels.ErrInvalidMessage},
		{"no mime", []channels.NewFile{{Property: "shot", Name: "a", Data: []byte("1")}}, channels.ErrInvalidMessage},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := h.Store.Append(ctx, k, withFiles("x", tc.files...))
			require.ErrorIs(t, err, tc.want)
		})
	}
	_, err := h.Store.Head(ctx, k)
	require.ErrorIs(t, err, channels.ErrNotFound, "a refused append creates no channel")

	// The same name in two properties is two files.
	other := newFile("a", "2")
	other.Property = "doc"
	got := mustAppend(t, h.Store, k, withFiles("x", newFile("a", "1"), other))
	require.Len(t, got.Files, 2)
}

func testFilesEditKeeps(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	got := mustAppend(t, h.Store, k, withFiles("before", newFile("a.txt", "kept-bytes")))
	edited, err := h.Store.Edit(ctx, k, got.Seq, map[string]any{"body": "after"}, nil, T0.Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, got.Files, edited.Files, "an edit keeps the files")
	_, data := readFile(t, h.Store, k, got.Seq, got.Files[0].ID)
	require.Equal(t, "kept-bytes", data)
}

func testFilesRemoveErases(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	const payload = "file-payload-91c3"
	keep := mustAppend(t, h.Store, k, withFiles("keep", newFile("k.txt", "kept-file-55a1")))
	gone := mustAppend(t, h.Store, k, withFiles("gone", newFile("secret-name-7d.txt", payload)))
	require.Contains(t, string(h.Raw(t)), payload)

	got, err := h.Store.Remove(ctx, k, gone.Seq, channels.Removal{By: Bob, At: T0.Add(time.Hour)})
	require.NoError(t, err)
	require.Empty(t, got.Files)
	requireNoFile(t, h.Store, k, gone.Seq, gone.Files[0].ID)
	raw := string(h.Raw(t))
	require.NotContains(t, raw, payload, "a removal erases the file bytes")
	require.NotContains(t, raw, "secret-name-7d", "a removal erases the file entry")

	_, data := readFile(t, h.Store, k, keep.Seq, keep.Files[0].ID)
	require.Equal(t, "kept-file-55a1", data, "other messages keep their files")

	// The next append does not reuse the removed message's files.
	next := mustAppend(t, h.Store, k, text("next"))
	require.Empty(t, next.Files)
}

func testFilesDeleteErases(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	const payload = "deleted-file-3e70"
	mustAppend(t, h.Store, k, withFiles("x", newFile("a.txt", payload)))
	head, err := h.Store.Head(ctx, k)
	require.NoError(t, err)
	require.NoError(t, h.Store.Delete(ctx, k, channels.Deletion{Expect: head.State, Rev: head.Rev, By: Bob,
		At: T0.Add(time.Hour)}))
	require.NotContains(t, string(h.Raw(t)), payload, "a delete erases the file bytes")

	// A new channel under the key starts at seq 1 with no files.
	m := mustAppend(t, h.Store, k, text("new"))
	require.Empty(t, m.Files)
}

func testFilesRekey(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	got := mustAppend(t, h.Store, key("OLD-1", "room"), withFiles("x", newFile("a.txt", "moved-bytes")))
	require.NoError(t, h.Store.Rekey(ctx, "OLD-1", "NEW-1", T0.Add(time.Hour)))
	_, data := readFile(t, h.Store, key("NEW-1", "room"), got.Seq, got.Files[0].ID)
	require.Equal(t, "moved-bytes", data)
	requireNoFile(t, h.Store, key("OLD-1", "room"), got.Seq, got.Files[0].ID)
}

func testFilesTombstonePurge(t *testing.T, h Harness) {
	t.Helper()
	ctx := context.Background()
	k := key("TKT-1", "room")
	const payload = "tombstoned-file-a2f4"
	got := mustAppend(t, h.Store, k, withFiles("x", newFile("a.txt", payload)))

	// An untombstone brings the files back.
	h1 := tombstone(t, h.Store, "TKT-1", "", T0.Add(time.Minute))
	requireNoFile(t, h.Store, k, got.Seq, got.Files[0].ID)
	require.NoError(t, h.Store.Untombstone(ctx, h1, T0.Add(2*time.Minute), nil))
	_, data := readFile(t, h.Store, k, got.Seq, got.Files[0].ID)
	require.Equal(t, payload, data)

	// A purge erases them.
	b := fileBatch(t, h.Store, "TKT-1", "", T0.Add(3*time.Minute), channels.Filing{Policy: channels.PolicyDelete})
	require.NoError(t, h.Store.Purge(ctx, b, nil, Bob, T0.Add(time.Hour)))
	require.NotContains(t, string(h.Raw(t)), payload, "a purge erases the file bytes")
}
