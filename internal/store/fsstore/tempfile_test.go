package fsstore_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// writeAged writes a file under root and sets its mtime to age ago.
func writeAged(t *testing.T, root, rel, data string, age time.Duration) string {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(t, os.WriteFile(p, []byte(data), 0o644))
	mtime := time.Now().Add(-age)
	require.NoError(t, os.Chtimes(p, mtime, mtime))
	return p
}

// AC3: opening a store removes stale temp files of interrupted writes and
// keeps fresh ones, which may be another process's write in flight. Every
// temp form is covered: the random SafeFS name, the old fixed SafeFS name,
// and the legacy ".new" name. Each case also holds a stale real entity
// and relation, which the sweep must leave alone.
func TestOpen_TempFileSweepHonoursGracePeriod(t *testing.T) {
	stale := storage.TempStaleAfter + time.Minute
	tests := []struct {
		name string
		rel  string
	}{
		{name: "entity random tmp", rel: "entities/requirements/REQ-2.md.0123456789abcdef.tmp"},
		{name: "entity fixed tmp", rel: "entities/requirements/REQ-2.md.tmp"},
		{name: "entity new", rel: "entities/requirements/REQ-2.md.new"},
		{name: "relation random tmp", rel: "relations/A--rel--B.md.0123456789abcdef.tmp"},
		{name: "relation fixed tmp", rel: "relations/A--rel--B.md.tmp"},
		{name: "relation new", rel: "relations/A--rel--B.md.new"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			root := t.TempDir()
			writeAged(t, root, "entities/requirements/REQ-1.md", "---\nid: REQ-1\ntype: requirement\n---\n", stale)
			writeAged(t, root, "entities/solutions/SOL-1.md", "---\nid: SOL-1\ntype: solution\n---\n", stale)
			writeAged(t, root, "relations/SOL-1--implements--REQ-1.md",
				"---\nfrom: SOL-1\nrelation: implements\nto: REQ-1\n---\n", stale)
			freshRel := strings.Replace(tc.rel, "2.md", "3.md", 1)
			freshRel = strings.Replace(freshRel, "--B.md", "--C.md", 1)
			stalePath := writeAged(t, root, tc.rel, "partial", stale)
			freshPath := writeAged(t, root, freshRel, "in flight", time.Second)

			s := openOSStore(t, root)
			defer s.Close()

			_, err := os.Stat(stalePath)
			require.ErrorIs(t, err, os.ErrNotExist, "stale temp file must be removed")
			_, err = os.Stat(freshPath)
			require.NoError(t, err, "fresh temp file must be kept")
			_, err = s.GetEntity(ctx, entity.Ref{ID: "REQ-1"})
			require.NoError(t, err, "stale real entity must be kept")
			_, err = s.GetRelation(ctx, entity.RelationKey{From: "SOL-1", Type: "implements", To: "REQ-1"})
			require.NoError(t, err, "stale real relation must be kept")
		})
	}
}

// Opening a store removes a stale attachment upload left by a crash and
// keeps a fresh one, which may be another process's upload. A stale real
// attachment is kept and indexed; neither upload is indexed.
func TestOpen_AttachmentTempSweepHonoursGracePeriod(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	stale := storage.TempStaleAfter + time.Minute
	writeAged(t, root, "entities/requirements/DOC-1.md", "---\nid: DOC-1\ntype: requirement\n---\n", stale)
	writeAged(t, root, "attachments/DOC-1/spec/a.txt", "real", stale)
	stalePath := writeAged(t, root, "attachments/DOC-1/spec/"+storage.AttachTempName(), "partial", stale)
	freshPath := writeAged(t, root, "attachments/DOC-1/spec/"+storage.AttachTempName(), "in flight", time.Second)

	s := openOSStore(t, root)
	defer s.Close()

	_, err := os.Stat(stalePath)
	require.ErrorIs(t, err, os.ErrNotExist, "stale upload must be removed")
	_, err = os.Stat(freshPath)
	require.NoError(t, err, "fresh upload must be kept")

	infos, err := s.ListFamilyAttachments(ctx, "DOC-1")
	require.NoError(t, err)
	require.Len(t, infos, 1, "only the real attachment is indexed")
	assert.Equal(t, "a.txt", infos[0].FileName)
}

// AC5: a temp file holds complete entity markdown just before its rename.
// The scan must not read it as an entity.
func TestOpen_ScanIgnoresTempFile(t *testing.T) {
	root := t.TempDir()
	body := "---\nid: REQ-9\ntype: requirement\n---\n"
	writeAged(t, root, "entities/requirements/REQ-1.md", "---\nid: REQ-1\ntype: requirement\n---\n", 0)
	writeAged(t, root, "entities/requirements/REQ-9.md.0123456789abcdef.tmp", body, 0)

	s := openOSStore(t, root)
	defer s.Close()

	ctx := context.Background()
	_, err := s.GetEntity(ctx, entity.Ref{ID: "REQ-1"})
	require.NoError(t, err)
	_, err = s.GetEntity(ctx, entity.Ref{ID: "REQ-9"})
	assert.ErrorIs(t, err, store.ErrNotFound)
}

// AC8: two processes attaching the same file name to the same entity at
// once both succeed. With one temp name per file name, one process's
// rename moved the shared temp file away and the other's rename failed.
func TestAttach_ConcurrentSameNameAcrossStores(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	seed := openOSStore(t, root)
	require.NoError(t, seed.CreateEntity(ctx, entity.New("DOC-1", "requirement")))
	require.NoError(t, seed.Close())

	a := openOSStore(t, root)
	defer a.Close()
	b := openOSStore(t, root)
	defer b.Close()

	payloads := []string{strings.Repeat("a", 64<<10), strings.Repeat("b", 32<<10)}
	for i := range 20 {
		var wg sync.WaitGroup
		errs := make([]error, 2)
		wg.Go(func() { errs[0] = a.AttachFamilyFile(ctx, "DOC-1", "spec", "a.txt", strings.NewReader(payloads[0])) })
		wg.Go(func() { errs[1] = b.AttachFamilyFile(ctx, "DOC-1", "spec", "a.txt", strings.NewReader(payloads[1])) })
		wg.Wait()
		require.NoErrorf(t, errs[0], "iteration %d: store a", i)
		require.NoErrorf(t, errs[1], "iteration %d: store b", i)

		got, err := os.ReadFile(filepath.Join(root, "attachments", "DOC-1", "spec", "a.txt"))
		require.NoError(t, err)
		require.Truef(t, string(got) == payloads[0] || string(got) == payloads[1],
			"iteration %d: attachment matches neither upload", i)
	}
}
