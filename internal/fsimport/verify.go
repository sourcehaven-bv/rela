package fsimport

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/canonical"
	"github.com/Sourcehaven-BV/rela/internal/comments"
	"github.com/Sourcehaven-BV/rela/internal/datamigration"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// verifyTarget compares the renamed target with the source, record by
// record. Entities and relations are compared by canonical hash after the
// same normalization the copy applied; store.VersionOf would not do, because
// it hashes Go types and the two backends decode values into different ones.
//
// Every mismatch is recorded; any one fails the run.
func verifyTarget(ctx context.Context, src *source, dst Target, kept importedSet, rep *Report) error {
	before := len(rep.Errors)
	v := verifier{src: src, dst: dst, kept: kept, rep: rep}
	v.entities(ctx)
	v.relations(ctx)
	v.attachments(ctx)
	v.comments(ctx)
	v.migrations(ctx)
	v.stateKeys(ctx)
	if n := len(rep.Errors) - before; n > 0 {
		return fmt.Errorf("verification found %d difference(s) between the source and the new project", n)
	}
	return nil
}

type verifier struct {
	src  *source
	dst  Target
	kept importedSet
	rep  *Report
}

func (v verifier) entities(ctx context.Context) {
	for e, err := range v.src.store.ListEntities(ctx, allEntities()) {
		if err != nil {
			v.rep.fail("verify: read entity: %v", err)
			continue
		}
		ref := entity.FormatStateRef(e.ID, e.Face)
		if e.IsLocked() {
			v.rep.fail("verify: entity %s is encrypted", ref)
			continue
		}
		want, err := normalizedEntity(v.src, e)
		if err != nil {
			v.rep.fail("verify: entity %s: %v", ref, err)
			continue
		}
		got, err := v.dst.Store.GetEntity(ctx, entity.Ref{ID: e.ID, Face: e.Face})
		if err != nil {
			v.rep.fail("verify: entity %s: %v", ref, err)
			continue
		}
		if canonical.HashEntity(*want.e) != canonical.HashEntity(*got) {
			v.rep.fail("verify: entity %s differs in the new database", ref)
		}
	}
}

func (v verifier) relations(ctx context.Context) {
	for r, err := range v.src.store.ListRelations(ctx, store.RelationQuery{}) {
		if err != nil {
			v.rep.fail("verify: read relation: %v", err)
			continue
		}
		name := relationName(r.Identity())
		if r.IsLocked() {
			v.rep.fail("verify: relation %s is encrypted", name)
			continue
		}
		want, err := normalizedRelation(v.src, r)
		if err != nil {
			v.rep.fail("verify: relation %s: %v", name, err)
			continue
		}
		got, err := v.dst.Store.GetRelation(ctx, r.Identity())
		if err != nil {
			v.rep.fail("verify: relation %s: %v", name, err)
			continue
		}
		if canonical.HashRelation(*want.r) != canonical.HashRelation(*got) {
			v.rep.fail("verify: relation %s differs in the new database", name)
		}
	}
}

func (v verifier) attachments(ctx context.Context) {
	families := map[string]bool{}
	for e, err := range v.src.store.ListEntities(ctx, allEntities()) {
		if err == nil {
			families[e.ID] = true
		}
	}
	for _, id := range sortedKeys(families) {
		infos, err := v.src.store.ListFamilyAttachments(ctx, id)
		if err != nil {
			v.rep.fail("verify: list attachments of %s: %v", id, err)
			continue
		}
		for _, a := range infos {
			name := attachmentName(a)
			want, err := hashAttachment(ctx, v.src.store, a, true)
			if err != nil {
				v.rep.fail("verify: attachment %s: %v", name, describeWriteErr(err))
				continue
			}
			got, err := hashAttachment(ctx, v.dst.Store, a, false)
			if err != nil {
				v.rep.fail("verify: attachment %s: %v", name, err)
				continue
			}
			if want != got {
				v.rep.fail("verify: attachment %s differs in the new database", name)
			}
		}
	}
}

// attachmentReader is the part of a store hashAttachment reads through.
type attachmentReader interface {
	ReadFamilyAttachment(ctx context.Context, entityID, property, fileName string) (io.ReadCloser, error)
}

func hashAttachment(
	ctx context.Context, st attachmentReader, a store.AttachmentInfo, refuseLocked bool,
) (string, error) {
	rc, err := st.ReadFamilyAttachment(ctx, a.EntityID, a.Property, a.FileName)
	if err != nil {
		return "", err
	}
	defer func() { _ = rc.Close() }()
	var r io.Reader = rc
	if refuseLocked {
		if r, err = lockedReader(rc); err != nil {
			return "", err
		}
	}
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (v verifier) comments(ctx context.Context) {
	keys, err := v.src.comments.ThreadKeys(ctx)
	if err != nil {
		v.rep.fail("verify: list comment threads: %v", err)
		return
	}
	for _, key := range keys {
		id, face, err := entity.ParseStateRef(key)
		if err != nil {
			continue
		}
		e, err := v.dst.Store.GetEntity(ctx, entity.Ref{ID: id, Face: face})
		if err != nil {
			continue // not imported; listed by the copy
		}
		target := comments.Target{Type: e.Type, ID: id, Face: face}
		want, err := v.src.comments.List(ctx, target)
		if err != nil {
			v.rep.fail("verify: comments of %s: %v", key, err)
			continue
		}
		got, err := v.dst.Comments.List(ctx, target)
		if err != nil {
			v.rep.fail("verify: comments of %s: %v", key, err)
			continue
		}
		if !sameJSON(utcComments(want), utcComments(got)) {
			v.rep.fail("verify: comments of %s differ in the new database", key)
		}
	}
}

func (v verifier) migrations(ctx context.Context) {
	if v.kept.keptMigrations {
		return
	}
	want, err := v.src.migState.Load(ctx)
	if err != nil {
		v.rep.fail("verify: read the applied-migration record: %v", err)
		return
	}
	got, err := v.dst.Migrations.Load(ctx)
	if err != nil {
		v.rep.fail("verify: read the applied-migration record: %v", err)
		return
	}
	if !sameJSON(utcState(want), utcState(got)) {
		v.rep.fail("verify: the applied-migration record differs in the new database")
	}
}

// stateKeys compares every state key the copy wrote.
func (v verifier) stateKeys(ctx context.Context) {
	keys, err := sourceStateKeys(v.src.root)
	if err != nil {
		v.rep.fail("verify: list state keys: %v", err)
		return
	}
	for _, key := range keys {
		if v.kept.keptState[key] {
			continue
		}
		want, err := v.src.state.Get(ctx, key)
		if err == nil && key == schedulerStateKey {
			want, err = filterSchedulerState(want)
		}
		if err != nil {
			v.rep.fail("verify: state %s: %v", key, err)
			continue
		}
		got, err := v.dst.State.Get(ctx, key)
		if err != nil {
			v.rep.fail("verify: state %s: %v", key, err)
			continue
		}
		if !bytes.Equal(want, got) {
			v.rep.fail("verify: state %s differs in the new database", key)
		}
	}
}

// utcComments puts every time in UTC: a database returns the same instant
// in a different zone than the YAML file stated it in.
func utcComments(list []comments.Comment) []comments.Comment {
	out := make([]comments.Comment, len(list))
	for i, c := range list {
		c.CreatedAt = c.CreatedAt.UTC()
		c.UpdatedAt = c.UpdatedAt.UTC()
		out[i] = c
	}
	// Backends order a thread differently (file order, or created_at then
	// id); the import keeps the comments, not their order.
	slices.SortFunc(out, func(a, b comments.Comment) int {
		if c := a.CreatedAt.Compare(b.CreatedAt); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
	return out
}

// utcState puts every time in UTC and compacts the projection, so two
// records that mean the same compare equal.
func utcState(st *datamigration.State) *datamigration.State {
	if st == nil {
		return nil
	}
	out := *st
	out.UpdatedAt = st.UpdatedAt.UTC()
	out.Applied = make([]datamigration.AppliedEntry, len(st.Applied))
	for i, a := range st.Applied {
		a.AppliedAt = a.AppliedAt.UTC()
		out.Applied[i] = a
	}
	var buf bytes.Buffer
	if json.Compact(&buf, st.Projection) == nil {
		out.Projection = buf.Bytes()
	}
	return &out
}

// sameJSON compares two values by their JSON form, which is how every
// database backend stores them: a time read back from a database is equal to
// the original in value but not in Go representation.
func sameJSON(a, b any) bool {
	ja, errA := json.Marshal(a)
	jb, errB := json.Marshal(b)
	return errA == nil && errB == nil && bytes.Equal(ja, jb)
}

// treeFingerprint summarizes every file below root except .git by path,
// size, mode and modification time. Two equal fingerprints mean nothing in
// the tree was written in between; it is how the import detects a source
// edited during the run.
//
// exclude lists slash-separated paths below root, each matched as a prefix,
// that the run itself writes: the database of an import into the project
// it reads.
func treeFingerprint(root string, exclude ...string) (string, error) {
	h := sha256.New()
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		if rel == ".git" {
			return fs.SkipDir
		}
		slashed := filepath.ToSlash(rel)
		for _, x := range exclude {
			if strings.HasPrefix(slashed, x) {
				return nil
			}
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(h, "%s\x00%d\x00%o\x00%d\n",
			strings.ReplaceAll(rel, string(os.PathSeparator), "/"),
			info.Size(), info.Mode(), info.ModTime().UnixNano())
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("read the source tree: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
