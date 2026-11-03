package filechannels

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/channels/channelstest"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

var carol = channels.Reader{User: "carol"}

// mentionsFailFS fails every write of a mentions.json while fail is set.
type mentionsFailFS struct {
	storage.FS
	fail bool
}

func (f *mentionsFailFS) WriteFile(p string, data []byte, perm fs.FileMode) error {
	if f.fail && strings.HasSuffix(p, "/"+mentionsFile) {
		return errors.New("injected mentions.json failure")
	}
	return f.FS.WriteFile(p, data, perm)
}

// mentioning is msg(body) mentioning targets as logins.
func mentioning(body string, targets ...string) channels.NewMessage {
	m := msg(body)
	for _, t := range targets {
		m.Mentions = append(m.Mentions, channels.Mention{Target: t, Form: channels.FormLogin})
	}
	return m
}

func listAddressed(t *testing.T, s *Store, r channels.Reader) []channels.Addressed {
	t.Helper()
	got, err := s.ListAddressed(context.Background(), r, channels.AddressAny, channels.MaxAddressedScan)
	require.NoError(t, err)
	return got
}

// TestAddressed_WriteOrder pins the write order of mentions.json (see
// mentionsFile): a failed row write after the log record does not fail an
// append, and a failed row write before it fails an edit that drops a
// target and a removal, leaving the message as it was.
func TestAddressed_WriteOrder(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ffs := &mentionsFailFS{FS: storage.NewMemFS()}
	s, err := newStore(ffs, "/project/.rela/channels", &countingLocker{}, SegmentSize)
	require.NoError(t, err)

	_, err = s.Append(ctx, key, mentioning("one", "carol"))
	require.NoError(t, err)

	ffs.fail = true
	got, err := s.Append(ctx, key, mentioning("two", "carol"))
	require.NoError(t, err, "a stored post is never reported as failed")
	require.Equal(t, int64(2), got.Seq)
	require.Len(t, listAddressed(t, s, carol), 1, "the lost row write left the file as it was")

	_, err = s.Edit(ctx, key, 1, map[string]any{"body": "dropped"}, nil, channelstest.T0)
	require.Error(t, err, "a drop that cannot be written fails the edit")
	m, err := s.Get(ctx, key, 1)
	require.NoError(t, err)
	require.Equal(t, "one", m.Props["body"])
	require.Len(t, m.Mentions, 1)

	_, err = s.Remove(ctx, key, 1, channels.Removal{By: channelstest.Bob, At: channelstest.T0})
	require.Error(t, err, "a removal whose rows cannot go fails")
	m, err = s.Get(ctx, key, 1)
	require.NoError(t, err)
	require.Nil(t, m.Removed)
	require.Len(t, listAddressed(t, s, carol), 1)

	ffs.fail = false
	_, err = s.Edit(ctx, key, 2, map[string]any{"body": "two again"},
		[]channels.Mention{{Target: "carol", Form: channels.FormLogin}}, channelstest.T0.Add(time.Hour))
	require.NoError(t, err)
	require.Len(t, listAddressed(t, s, carol), 1,
		"an edit that keeps the target writes no row: the lost row stays lost")
}

// TestAddressed_DamagedFile pins that a damaged mentions.json reads as no
// rows, and that the next row write replaces it.
func TestAddressed_DamagedFile(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mfs := storage.NewMemFS()
	dir := "/project/.rela/channels"
	s, err := newStore(mfs, dir, &countingLocker{}, SegmentSize)
	require.NoError(t, err)
	_, err = s.Append(ctx, key, mentioning("one", "carol"))
	require.NoError(t, err)
	rfs, err := storage.NewRootedFS(mfs, dir)
	require.NoError(t, err)
	require.NoError(t, rfs.WriteFile(mentionsKey(key), []byte("{not json"), filePerm))

	require.Empty(t, listAddressed(t, s, carol), "a damaged file holds no rows")
	_, err = s.Append(ctx, key, mentioning("two", "carol"))
	require.NoError(t, err)
	rows := listAddressed(t, s, carol)
	require.Len(t, rows, 1)
	require.Equal(t, int64(2), rows[0].Seq)
}

// TestAddressed_EditWritesOnlyAdded pins that an edit writes only the
// rows of the targets it adds: when the file is lost, an edit that keeps a
// target does not write its row again and the reply row does not come
// back, as on the SQL tiers. An added target gets its row at the edit time.
func TestAddressed_EditWritesOnlyAdded(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mfs := storage.NewMemFS()
	dir := "/project/.rela/channels"
	s, err := newStore(mfs, dir, &countingLocker{}, SegmentSize)
	require.NoError(t, err)
	parent := msg("parent")
	parent.Author = channelstest.Bob
	_, err = s.Append(ctx, key, parent)
	require.NoError(t, err)
	reply := mentioning("reply", "carol")
	reply.ReplyTo = 1
	_, err = s.Append(ctx, key, reply)
	require.NoError(t, err)

	rfs, err := storage.NewRootedFS(mfs, dir)
	require.NoError(t, err)
	require.NoError(t, rfs.Remove(mentionsKey(key)))
	require.Empty(t, listAddressed(t, s, carol))

	edited := channelstest.T0.Add(time.Hour)
	_, err = s.Edit(ctx, key, 2, map[string]any{"body": "edited"}, []channels.Mention{
		{Target: "carol", Form: channels.FormLogin}, {Target: "dave", Form: channels.FormLogin},
	}, edited)
	require.NoError(t, err)
	require.Empty(t, listAddressed(t, s, carol), "a kept target gets no row back")
	require.Empty(t, listAddressed(t, s, channels.ReaderOf(channelstest.Bob)), "the reply row does not come back")
	rows := listAddressed(t, s, channels.Reader{User: "dave"})
	require.Len(t, rows, 1)
	require.Equal(t, channels.NormTime(edited), rows[0].At)
}

// TestAddressed_FileCap pins [channels.MaxAddressedPerFile] in makeRoom: a
// full file frees the oldest read row across targets, else the poster's
// own oldest row, else refuses the new row.
func TestAddressed_FileCap(t *testing.T) {
	t.Parallel()
	full := func(author func(i int) string) []mentionRow {
		rows := make([]mentionRow, 0, channels.MaxAddressedPerFile)
		for i := range channels.MaxAddressedPerFile {
			rows = append(rows, mentionRow{
				Seq: int64(i + 1), Target: fmt.Sprint("user-", i%50), Kind: channels.AddressMention,
				Author: channels.Author{User: author(i)},
			})
		}
		return rows
	}
	byBob := func(int) string { return "bob" }
	nr := mentionRow{Seq: channels.MaxAddressedPerFile + 1, Target: "new", Kind: channels.AddressMention,
		Author: channels.Author{User: "alice"}}
	none := func(string) []int64 { return nil }

	_, ok := makeRoom(full(byBob), nr, none)
	require.False(t, ok, "a file full of another poster's unread rows refuses the row")

	rows, ok := makeRoom(full(func(i int) string {
		if i == 7 {
			return "alice"
		}
		return "bob"
	}), nr, none)
	require.True(t, ok)
	require.Len(t, rows, channels.MaxAddressedPerFile-1)
	for _, r := range rows {
		require.NotEqual(t, int64(8), r.Seq, "the poster's own oldest row went")
	}

	readUpTo := func(target string) []int64 {
		if target == "user-3" {
			return []int64{100}
		}
		return nil
	}
	rows, ok = makeRoom(full(byBob), nr, readUpTo)
	require.True(t, ok)
	for _, r := range rows {
		require.NotEqual(t, int64(4), r.Seq, "the oldest read row went")
	}
}
