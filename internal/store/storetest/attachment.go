package storetest

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

type failReader struct{}

func (f *failReader) Read([]byte) (int, error) {
	return 0, errors.New("read failed")
}

// constReader yields an endless stream of the same byte. Used (with
// io.LimitReader) to feed an oversize attachment without allocating the
// full payload in the test.
type constReader byte

func (c constReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(c)
	}
	return len(p), nil
}

// RunAttachmentTests runs attachment conformance tests.
func RunAttachmentTests(t *testing.T, f Factory) {
	t.Run("AttachAndRead", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("T-1", "ticket")))

		err := s.AttachFile(ctx(), "T-1", "screenshot", "bug.png", strings.NewReader("image data"))
		require.NoError(t, err)

		rc, err := s.ReadAttachment(ctx(), "T-1", "screenshot", "bug.png")
		require.NoError(t, err)
		defer rc.Close()

		data, err := io.ReadAll(rc)
		require.NoError(t, err)
		assert.Equal(t, "image data", string(data))
	})

	t.Run("AttachEntityNotFound", func(t *testing.T) {
		s := f(t)
		err := s.AttachFile(ctx(), "NOPE", "prop", "f.txt", strings.NewReader("x"))
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("ReadNotFound", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("T-1", "ticket")))

		_, err := s.ReadAttachment(ctx(), "T-1", "noprop", "x")
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("Delete", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("T-1", "ticket")))
		require.NoError(t, s.AttachFile(ctx(), "T-1", "screenshot", "bug.png", strings.NewReader("data")))

		err := s.DeleteAttachment(ctx(), "T-1", "screenshot", "bug.png")
		require.NoError(t, err)

		_, err = s.ReadAttachment(ctx(), "T-1", "screenshot", "bug.png")
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("DeleteNotFound", func(t *testing.T) {
		s := f(t)
		err := s.DeleteAttachment(ctx(), "T-1", "noprop", "x")
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("List", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("T-1", "ticket")))
		require.NoError(t, s.AttachFile(ctx(), "T-1", "screenshot", "bug.png", strings.NewReader("img")))
		require.NoError(t, s.AttachFile(ctx(), "T-1", "log", "app.log", strings.NewReader("log data here")))

		infos, err := s.ListAttachments(ctx(), "T-1")
		require.NoError(t, err)
		assert.Len(t, infos, 2)

		var found bool
		for _, info := range infos {
			if info.Property == "log" {
				assert.Equal(t, "T-1", info.EntityID)
				assert.Equal(t, "app.log", info.FileName)
				assert.Equal(t, int64(13), info.Size)
				found = true
			}
		}
		assert.True(t, found, "expected to find 'log' attachment")
	})

	t.Run("ListEmpty", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("T-1", "ticket")))

		infos, err := s.ListAttachments(ctx(), "T-1")
		require.NoError(t, err)
		assert.Empty(t, infos)
	})

	t.Run("RejectsEmptyProperty", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("T-1", "ticket")))

		err := s.AttachFile(ctx(), "T-1", "", "f.txt", strings.NewReader("x"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty property")
	})

	t.Run("RejectsSlashInProperty", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("E-1", "t")))

		err := s.AttachFile(ctx(), "E-1", "some/prop", "f.txt", strings.NewReader("data"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "slash")

		err = s.AttachFile(ctx(), "E-1", "screenshot", "f.png", strings.NewReader("data"))
		require.NoError(t, err)
	})

	t.Run("ReaderError", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("T-1", "ticket")))

		err := s.AttachFile(ctx(), "T-1", "doc", "f.txt", &failReader{})
		assert.Error(t, err)
	})

	t.Run("ListEntityNotFound", func(t *testing.T) {
		s := f(t)
		_, err := s.ListAttachments(ctx(), "NOPE")
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("RejectsOversize", func(t *testing.T) {
		// Every backend enforces store.MaxAttachmentBytes as a backstop so
		// no storage path is ever unbounded. Feed just over the cap via a
		// constant reader (no multi-MiB allocation in the test) and assert
		// the shared sentinel.
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("T-1", "ticket")))

		oversize := io.LimitReader(constReader('a'), store.MaxAttachmentBytes+1)
		err := s.AttachFile(ctx(), "T-1", "screenshot", "big.bin", oversize)
		assert.ErrorIs(t, err, store.ErrAttachmentTooLarge)

		// The rejected write must not leave a half-attachment behind.
		infos, listErr := s.ListAttachments(ctx(), "T-1")
		require.NoError(t, listErr)
		assert.Empty(t, infos, "oversize attach must not persist a partial attachment")
	})

	t.Run("OversizeReplaceKeepsExisting", func(t *testing.T) {
		// A failed (oversize) replace must NOT destroy the existing valid
		// attachment — the new bytes are written to the side and only
		// swapped in on success.
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("T-1", "ticket")))
		require.NoError(t, s.AttachFile(ctx(), "T-1", "screenshot", "ok.png", strings.NewReader("original")))

		// Same property, oversize — must be rejected.
		oversize := io.LimitReader(constReader('a'), store.MaxAttachmentBytes+1)
		err := s.AttachFile(ctx(), "T-1", "screenshot", "huge.bin", oversize)
		assert.ErrorIs(t, err, store.ErrAttachmentTooLarge)

		// The original attachment must still be intact and readable.
		rc, readErr := s.ReadAttachment(ctx(), "T-1", "screenshot", "ok.png")
		require.NoError(t, readErr)
		defer rc.Close()
		data, _ := io.ReadAll(rc)
		assert.Equal(t, "original", string(data), "failed replace must not destroy the existing attachment")
	})

	t.Run("AppendsMultipleFilesPerProperty", func(t *testing.T) {
		// Two DIFFERENT file names on the same property now COEXIST (the
		// store appends; it no longer overwrites by property). Per-file
		// cap/replace policy lives in the write path, not the store.
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("T-1", "ticket")))
		require.NoError(t, s.AttachFile(ctx(), "T-1", "doc", "v1.txt", strings.NewReader("version 1")))
		require.NoError(t, s.AttachFile(ctx(), "T-1", "doc", "v2.txt", strings.NewReader("version 2")))

		infos, _ := s.ListAttachments(ctx(), "T-1")
		assert.Len(t, infos, 2, "two differently-named files on one property must coexist")

		// Each is readable by its own filename.
		for name, want := range map[string]string{"v1.txt": "version 1", "v2.txt": "version 2"} {
			rc, err := s.ReadAttachment(ctx(), "T-1", "doc", name)
			require.NoError(t, err)
			data, _ := io.ReadAll(rc)
			rc.Close()
			assert.Equal(t, want, string(data))
		}
	})

	t.Run("SameNameReplacesOnlyThatFile", func(t *testing.T) {
		// Re-attaching the SAME file name replaces just that one file;
		// sibling files on the property are untouched.
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("T-1", "ticket")))
		require.NoError(t, s.AttachFile(ctx(), "T-1", "doc", "a.txt", strings.NewReader("a1")))
		require.NoError(t, s.AttachFile(ctx(), "T-1", "doc", "b.txt", strings.NewReader("b1")))
		require.NoError(t, s.AttachFile(ctx(), "T-1", "doc", "a.txt", strings.NewReader("a2"))) // replace a

		infos, _ := s.ListAttachments(ctx(), "T-1")
		assert.Len(t, infos, 2, "same-name re-attach must not add a row")

		rcA, _ := s.ReadAttachment(ctx(), "T-1", "doc", "a.txt")
		dataA, _ := io.ReadAll(rcA)
		rcA.Close()
		assert.Equal(t, "a2", string(dataA), "a.txt should be replaced")

		rcB, _ := s.ReadAttachment(ctx(), "T-1", "doc", "b.txt")
		dataB, _ := io.ReadAll(rcB)
		rcB.Close()
		assert.Equal(t, "b1", string(dataB), "b.txt must be untouched")
	})

	t.Run("PerFileDeleteLeavesSiblings", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("T-1", "ticket")))
		require.NoError(t, s.AttachFile(ctx(), "T-1", "doc", "a.txt", strings.NewReader("a")))
		require.NoError(t, s.AttachFile(ctx(), "T-1", "doc", "b.txt", strings.NewReader("b")))

		require.NoError(t, s.DeleteAttachment(ctx(), "T-1", "doc", "a.txt"))

		_, err := s.ReadAttachment(ctx(), "T-1", "doc", "a.txt")
		assert.ErrorIs(t, err, store.ErrNotFound)
		rc, err := s.ReadAttachment(ctx(), "T-1", "doc", "b.txt")
		require.NoError(t, err, "deleting one file must leave its siblings")
		rc.Close()
	})

	t.Run("RejectsBadFileName", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("T-1", "ticket")))

		for _, bad := range []string{"", "a/b.txt", "a\\b.txt", "a\x00b", "..", "."} {
			err := s.AttachFile(ctx(), "T-1", "doc", bad, strings.NewReader("x"))
			assert.Error(t, err, "file name %q should be rejected", bad)
		}
		// A clean name still works.
		require.NoError(t, s.AttachFile(ctx(), "T-1", "doc", "ok.txt", strings.NewReader("x")))
	})

	t.Run("FileNameEndingInNewRoundTrips", func(t *testing.T) {
		// A file literally named "*.new" must store and read back — the temp
		// marker must not collide with a valid user filename (RR-BN2MDO).
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("T-1", "ticket")))
		require.NoError(t, s.AttachFile(ctx(), "T-1", "doc", "report.new", strings.NewReader("payload")))

		rc, err := s.ReadAttachment(ctx(), "T-1", "doc", "report.new")
		require.NoError(t, err)
		defer rc.Close()
		data, _ := io.ReadAll(rc)
		assert.Equal(t, "payload", string(data))

		infos, _ := s.ListAttachments(ctx(), "T-1")
		require.Len(t, infos, 1)
		assert.Equal(t, "report.new", infos[0].FileName)
	})

	t.Run("RenameMovesAttachments", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("T-1", "ticket")))
		require.NoError(t, s.AttachFile(ctx(), "T-1", "spec", "doc.pdf", strings.NewReader("pdf bytes")))
		require.NoError(t, s.AttachFile(ctx(), "T-1", "spec", "extra.txt", strings.NewReader("extra")))

		_, err := s.RenameEntity(ctx(), "T-1", "T-2")
		require.NoError(t, err)

		_, err = s.ListAttachments(ctx(), "T-1")
		assert.ErrorIs(t, err, store.ErrNotFound)

		infos, err := s.ListAttachments(ctx(), "T-2")
		require.NoError(t, err)
		require.Len(t, infos, 2, "all files on the property move with the rename")

		rc, err := s.ReadAttachment(ctx(), "T-2", "spec", "doc.pdf")
		require.NoError(t, err)
		defer rc.Close()
		got, _ := io.ReadAll(rc)
		assert.Equal(t, "pdf bytes", string(got))
	})

	t.Run("DeleteCascadesAttachments", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("T-1", "ticket")))
		require.NoError(t, s.AttachFile(ctx(), "T-1", "spec", "doc.pdf", strings.NewReader("pdf bytes")))

		_, err := s.DeleteEntity(ctx(), "T-1", false)
		require.NoError(t, err)

		// Re-create with the same ID; stale attachments must not resurrect.
		require.NoError(t, s.CreateEntity(ctx(), entity.New("T-1", "ticket")))
		infos, err := s.ListAttachments(ctx(), "T-1")
		require.NoError(t, err)
		assert.Empty(t, infos)
	})
}
