package storetest

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failReader struct{}

func (f *failReader) Read([]byte) (int, error) {
	return 0, errors.New("read failed")
}

// RunAttachmentTests runs attachment conformance tests.
func RunAttachmentTests(t *testing.T, f Factory) {
	t.Run("AttachAndRead", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("T-1", "ticket")))

		err := s.AttachFile(ctx(), "T-1", "screenshot", "bug.png", strings.NewReader("image data"))
		require.NoError(t, err)

		rc, err := s.ReadAttachment(ctx(), "T-1", "screenshot")
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

		_, err := s.ReadAttachment(ctx(), "T-1", "noprop")
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("Delete", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("T-1", "ticket")))
		require.NoError(t, s.AttachFile(ctx(), "T-1", "screenshot", "bug.png", strings.NewReader("data")))

		err := s.DeleteAttachment(ctx(), "T-1", "screenshot")
		require.NoError(t, err)

		_, err = s.ReadAttachment(ctx(), "T-1", "screenshot")
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("DeleteNotFound", func(t *testing.T) {
		s := f(t)
		err := s.DeleteAttachment(ctx(), "T-1", "noprop")
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

	t.Run("OverwritesExisting", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("T-1", "ticket")))
		require.NoError(t, s.AttachFile(ctx(), "T-1", "doc", "v1.txt", strings.NewReader("version 1")))
		require.NoError(t, s.AttachFile(ctx(), "T-1", "doc", "v2.txt", strings.NewReader("version 2")))

		rc, err := s.ReadAttachment(ctx(), "T-1", "doc")
		require.NoError(t, err)
		defer rc.Close()
		data, _ := io.ReadAll(rc)
		assert.Equal(t, "version 2", string(data))

		infos, _ := s.ListAttachments(ctx(), "T-1")
		assert.Len(t, infos, 1)
		assert.Equal(t, "v2.txt", infos[0].FileName)
	})
}
