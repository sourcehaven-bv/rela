package fsimport

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/Sourcehaven-BV/rela/internal/app"
	"github.com/Sourcehaven-BV/rela/internal/comments/filecomments"
	"github.com/Sourcehaven-BV/rela/internal/datamigration"
	"github.com/Sourcehaven-BV/rela/internal/datamigration/filemigstate"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// source is the filesystem project, opened read-only.
type source struct {
	root     string
	fs       storage.FS
	meta     *metamodel.Metamodel
	store    store.Store
	comments *filecomments.Store
	state    state.KV
	migState datamigration.StateStore
}

// openSource opens every reader the import needs over a read-only view of
// root, so no code path in them can write to the source.
func openSource(ctx context.Context, root string) (*source, error) {
	ro, err := storage.NewReadOnlyFS(storage.NewOsFS())
	if err != nil {
		return nil, err
	}
	paths, err := project.At(root, ro)
	if err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}
	meta, _, err := metamodel.NewFSLoader(ro, paths.SchemaPath).Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("source schema: %w", err)
	}
	if len(meta.Entities) == 0 {
		return nil, errors.New("source schema declares no entity types")
	}
	st, err := (&app.FSFactory{FS: ro, Paths: paths, ReadOnly: true}).OpenStore(meta)
	if err != nil {
		return nil, fmt.Errorf("source store: %w", err)
	}
	src := &source{root: root, fs: ro, meta: meta, store: st}

	if src.comments, err = filecomments.New(ro, filepath.Join(root, ".rela", "comments")); err != nil {
		src.close()
		return nil, err
	}
	rooted, err := storage.NewRootedFS(ro, filepath.Join(root, ".rela"))
	if err != nil {
		src.close()
		return nil, err
	}
	src.state = state.NewFSKV(rooted)
	files, err := filemigstate.New(ro, root)
	if err != nil {
		src.close()
		return nil, err
	}
	// The same bridge appbuild wraps every tier in, so a project that still
	// carries the pre-TKT-XCJ0Y2 marker reads as the record it stands for.
	if src.migState, err = datamigration.NewLegacyBridge(files, src.state); err != nil {
		src.close()
		return nil, err
	}
	return src, nil
}

func (s *source) close() {
	if s.store != nil {
		_ = s.store.Close()
	}
}

// entityProps returns the declared properties of an entity type.
func (s *source) entityProps(typ string) map[string]metamodel.PropertyDef {
	return s.meta.Entities[typ].Properties
}

func (s *source) relationProps(typ string) map[string]metamodel.PropertyDef {
	return s.meta.Relations[typ].Properties
}

// allEntities is the query for every face of every entity.
func allEntities() store.EntityQuery {
	return store.EntityQuery{Faces: store.AllFaces()}
}

// checkLocked refuses a source holding git-crypt-encrypted content, listing
// every locked entity, relation and attachment. Importing a locked record
// would store its ciphertext stub as if it were the content.
//
// The copy and the verification check again, row by row: this pre-scan only
// makes the common case fail before anything is written.
func (s *source) checkLocked(ctx context.Context, rep *Report) error {
	var problems []string
	families := map[string]bool{}
	// A row that fails to read is skipped here; the copy reports it.
	for e, err := range s.store.ListEntities(ctx, allEntities()) {
		if err != nil {
			continue
		}
		if e.IsLocked() {
			problems = append(problems, "entity "+entity.FormatStateRef(e.ID, e.Face)+" is encrypted")
		}
		families[e.ID] = true
	}
	for r, err := range s.store.ListRelations(ctx, store.RelationQuery{}) {
		if err != nil {
			continue
		}
		if r.IsLocked() {
			problems = append(problems, "relation "+relationName(r.Identity())+" is encrypted")
		}
	}
	for _, id := range sortedKeys(families) {
		infos, err := s.store.ListFamilyAttachments(ctx, id)
		if err != nil {
			problems = append(problems, fmt.Sprintf("list attachments of %s: %v", id, err))
			continue
		}
		for _, a := range infos {
			locked, err := s.attachmentLocked(ctx, a)
			if err != nil {
				problems = append(problems, fmt.Sprintf("attachment %s: %v", attachmentName(a), err))
				continue
			}
			if locked {
				problems = append(problems, "attachment "+attachmentName(a)+" is encrypted")
			}
		}
	}
	if len(problems) > 0 {
		rep.Errors = append(rep.Errors, problems...)
		return fmt.Errorf("%d item(s) in the source are encrypted or unreadable; "+
			"unlock encrypted content with git-crypt first", len(problems))
	}
	return nil
}

func (s *source) attachmentLocked(ctx context.Context, a store.AttachmentInfo) (bool, error) {
	rc, err := s.store.ReadFamilyAttachment(ctx, a.EntityID, a.Property, a.FileName)
	if err != nil {
		return false, fmt.Errorf("read attachment %s: %w", attachmentName(a), err)
	}
	defer func() { _ = rc.Close() }()
	head := make([]byte, len(gitCryptMagic))
	n, err := io.ReadFull(rc, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("read attachment %s: %w", attachmentName(a), err)
	}
	return isGitCrypt(head[:n]), nil
}

// gitCryptMagic is the header git-crypt puts on every encrypted file. fsstore
// detects it on markdown; attachments are bytes it passes through unread.
var gitCryptMagic = []byte("\x00GITCRYPT\x00")

func isGitCrypt(b []byte) bool { return bytes.HasPrefix(b, gitCryptMagic) }

// lockedReader fails the read when the stream starts with the git-crypt
// header, so the copy refuses encrypted bytes that appeared after the
// pre-scan.
func lockedReader(r io.Reader) (io.Reader, error) {
	br := bufio.NewReaderSize(r, len(gitCryptMagic))
	head, err := br.Peek(len(gitCryptMagic))
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, bufio.ErrBufferFull) {
		return nil, err
	}
	if isGitCrypt(head) {
		return nil, errEncrypted
	}
	return br, nil
}

var errEncrypted = errors.New("is encrypted")

func relationName(k entity.RelationKey) string {
	return entity.FormatStateRef(k.From, k.FromFace) + "--" + k.Type + "--" + k.To
}

func attachmentName(a store.AttachmentInfo) string {
	return a.EntityID + "/" + a.Property + "/" + a.FileName
}
