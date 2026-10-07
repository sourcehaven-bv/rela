//go:build sqlite

package appbuild

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/app"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/fsimport"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
)

// markdownDataDirs are the directories a markdown project keeps its data
// in. [ExportMarkdownData] refuses a target that already has any of them.
var markdownDataDirs = []string{"entities", "relations", "attachments"}

// DataSummary counts what an import or export copied.
type DataSummary struct {
	Entities    int
	Relations   int
	Attachments int
}

func (s DataSummary) String() string {
	return fmt.Sprintf("%d entities, %d relations, %d attachments", s.Entities, s.Relations, s.Attachments)
}

// DataImportOptions tunes [ImportMarkdownData].
type DataImportOptions struct {
	// Force imports into a database that already holds entities. An
	// imported id that collides with a stored one still fails the import.
	Force bool
	// Audit receives the run's single audit record.
	//
	// Nil: rejected. The import bypasses the entitymanager, so this record
	// is the only trace of it in the audit log.
	Audit audit.Audit
}

// ImportMarkdownData copies the data of the markdown project in fromDir
// into the project's SQLite database: every entity, relation and
// attachment, the comment threads, the applied-migration record and the
// runtime state. The copy itself is [fsimport.Copy], the engine behind
// `rela db import-fs`, so both imports refuse, normalize, report and verify
// alike; this function adds the in-place parts.
//
// This is a raw-store write, sanctioned on the same terms as perf seeding:
// the trust boundary is the operator shell, every row is attributed to the
// "fs-import" tool, the run leaves one audit record, and it refuses a
// database that already holds entities unless opts.Force is set. No
// automation, validation or ACL runs, so the data arrives exactly as the
// files hold it.
//
// The copy is ONE transaction, and the state, comment and migration stores
// write on that transaction's connection. SQLite rolls it back on any error,
// so a failed import leaves the database as it was and can simply be
// re-run. The database admits one process, so nothing else waits on that
// lock.
//
// The schema is read from fromDir, falling back to the copy the database
// carries. Ids, properties, bodies, faces and relation tails are kept.
// Timestamps are not: the store stamps its own write time.
//
// The report is returned on failure too, as far as the run got, so the
// caller can list every problem.
func ImportMarkdownData(
	ctx context.Context, fsys storage.FS, paths *project.Context, fromDir string, opts DataImportOptions,
) (*fsimport.Report, error) {
	if opts.Audit == nil {
		return nil, errors.New("appbuild: ImportMarkdownData requires an audit sink")
	}
	db, err := openDatabase(ctx, Config{Paths: paths})
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()

	meta, err := dataMetamodel(ctx, fsys, paths, fromDir, db)
	if err != nil {
		return nil, err
	}
	dst, err := sqlitestore.New(db)
	if err != nil {
		return nil, err
	}
	defer func() { _ = dst.Close() }()

	existing, err := dst.CountEntities(ctx, store.EntityQuery{Faces: store.AllFaces()})
	if err != nil {
		return nil, fmt.Errorf("count entities: %w", err)
	}
	if existing > 0 && !opts.Force {
		return nil, fmt.Errorf(
			"the database already holds %d entities; importing needs an empty database (or --force)", existing)
	}

	p := principal.From(ctx)
	start := time.Now()
	var rep *fsimport.Report
	err = dst.Tx(ctx, func(view store.Store) error {
		conn, ok := sqlitestore.TxConn(view)
		if !ok {
			return errors.New("appbuild: the import transaction has no connection")
		}
		svc, svcErr := openDBServicesOn(conn)
		if svcErr != nil {
			return svcErr
		}
		var copyErr error
		rep, copyErr = fsimport.Copy(ctx, fsimport.CopyOptions{
			Source: fromDir,
			Meta:   meta,
			Target: fsimport.Target{
				Store: view, State: svc.kv, Comments: svc.comments, Migrations: svc.migState,
			},
			Exclude:   databaseFilesBelow(fromDir, paths),
			Principal: p,
		})
		return copyErr
	})
	summary := fmt.Sprintf("markdown import from %s: %s in %s",
		fromDir, importCounts(rep), time.Since(start).Round(time.Millisecond))
	if err != nil {
		// The transaction rolled back, so nothing was written.
		summary = fmt.Sprintf("markdown import from %s FAILED, nothing written: %s", fromDir, err)
	}
	opts.Audit.Record(audit.Record{
		Time:      time.Now().UTC(),
		Op:        audit.OpFSImport,
		Principal: p,
		Summary:   summary,
	})
	if err != nil {
		return rep, fmt.Errorf("import: %w", err)
	}
	return rep, nil
}

// importCounts summarizes what a report says was copied.
func importCounts(rep *fsimport.Report) string {
	if rep == nil {
		return "nothing"
	}
	return fmt.Sprintf("%d entities, %d relations, %d attachments, %d comments, %d state keys",
		rep.Entities, rep.Relations, rep.Attachments, rep.Comments, rep.StateKeys)
}

// databaseFilesBelow returns the database's path relative to dir when the
// database lies inside dir: an import of a project's own markdown files
// writes there while it reads, and the check that the source did not
// change must not count that.
func databaseFilesBelow(dir string, paths *project.Context) []string {
	rel, err := filepath.Rel(dir, DatabasePath(paths))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil
	}
	return []string{filepath.ToSlash(rel)}
}

// ExportMarkdownData writes every entity, relation and attachment in the
// project's SQLite database as a markdown project into toDir. Together with
// [DumpProjectConfig] the result is a project the filesystem build opens.
//
// It refuses before writing anything when toDir already has an entities,
// relations or attachments directory: merging into existing data could
// collide with, or silently sit beside, what is there. A copy that fails
// part way leaves those directories behind, and its error says to remove
// them.
//
// The schema used to lay out the files is the project's, read through the
// same layered config the server reads. Timestamps are not kept: the files
// get the time of the export.
func ExportMarkdownData(
	ctx context.Context, fsys storage.FS, paths *project.Context, toDir string,
) (DataSummary, error) {
	for _, name := range markdownDataDirs {
		target := filepath.Join(toDir, name)
		if _, err := os.Lstat(target); err == nil {
			return DataSummary{}, fmt.Errorf("%s already exists; export the data into a directory without it", target)
		} else if !errors.Is(err, os.ErrNotExist) {
			return DataSummary{}, err
		}
	}

	db, err := openExistingDatabase(ctx, paths)
	if err != nil {
		return DataSummary{}, err
	}
	defer func() { _ = db.Close() }()
	meta, err := dataMetamodel(ctx, fsys, paths, paths.Root, db)
	if err != nil {
		return DataSummary{}, err
	}
	src, err := sqlitestore.New(db)
	if err != nil {
		return DataSummary{}, err
	}
	defer func() { _ = src.Close() }()

	if mkErr := fsys.MkdirAll(toDir, 0o755); mkErr != nil {
		return DataSummary{}, mkErr
	}
	dst, err := (&app.FSFactory{FS: fsys, Paths: &project.Context{Root: toDir}}).OpenStore(meta)
	if err != nil {
		return DataSummary{}, fmt.Errorf("open markdown target %s: %w", toDir, err)
	}
	defer func() { _ = dst.Close() }()
	sum, err := copyData(ctx, src, dst)
	if err != nil {
		// The refusal above would block a plain retry, so say what to remove.
		return sum, fmt.Errorf("%w; the export is incomplete: remove %s from %s before trying again",
			err, strings.Join(markdownDataDirs, ", "), toDir)
	}
	return sum, nil
}

// dataMetamodel loads the schema the data is laid out by: the files in dir
// first, then the config the database carries, the same order the server
// reads them in.
func dataMetamodel(
	ctx context.Context, fsys storage.FS, paths *project.Context, dir string, db *sqlitedb.DB,
) (*metamodel.Metamodel, error) {
	loader, err := layerProjectConfig(dir, db)
	if err != nil {
		return nil, err
	}
	at := *paths
	at.Root = dir
	at.SchemaPath = filepath.Join(dir, project.SchemaFile)
	if path, _, found := project.SchemaFileAt(dir, fsys); found {
		at.SchemaPath = path
	}
	meta, err := loadMetamodel(ctx, Config{FS: fsys, Paths: &at, projectConfig: loader})
	if err != nil {
		return nil, fmt.Errorf("load schema: %w", err)
	}
	return meta, nil
}

// copyData copies every entity state, then every relation, then every
// attachment from src to dst. Entities go first because a relation and an
// attachment both need their entity to exist.
func copyData(ctx context.Context, src, dst store.Store) (DataSummary, error) {
	var sum DataSummary
	ids := map[string]bool{}
	var order []string
	for e, err := range src.ListEntities(ctx, store.EntityQuery{Faces: store.AllFaces()}) {
		if err != nil {
			return sum, fmt.Errorf("read entities: %w", err)
		}
		if e.IsLocked() {
			// An unreadable file (git-crypt without the key) would be copied
			// as an empty shell, and the shell would then be the only copy.
			return sum, fmt.Errorf("entity %s has unreadable fields; unlock the files before copying",
				entity.FormatStateRef(e.ID, e.Face))
		}
		if err := dst.CreateEntity(ctx, &entity.Entity{
			ID: e.ID, Type: e.Type, Face: e.Face, Properties: e.Properties, Content: e.Content,
		}); err != nil {
			return sum, fmt.Errorf("create %s: %w", entity.FormatStateRef(e.ID, e.Face), err)
		}
		sum.Entities++
		if !ids[e.ID] {
			ids[e.ID] = true
			order = append(order, e.ID)
		}
	}

	for r, err := range src.ListRelations(ctx, store.RelationQuery{}) {
		if err != nil {
			return sum, fmt.Errorf("read relations: %w", err)
		}
		if r.IsLocked() {
			return sum, fmt.Errorf("relation %s --%s--> %s has unreadable fields; unlock the files before copying",
				r.From, r.Type, r.To)
		}
		if r.Type == "" {
			return sum, fmt.Errorf("relation %s --> %s has no relation type; its file needs a 'relation:' key",
				r.From, r.To)
		}
		data := &store.RelationData{Properties: r.Properties, Content: r.Content}
		if _, err := dst.CreateRelation(ctx, r.Identity(), data); err != nil {
			return sum, fmt.Errorf("create %s --%s--> %s: %w", r.From, r.Type, r.To, err)
		}
		sum.Relations++
	}

	for _, id := range order {
		n, err := copyAttachments(ctx, src, dst, id)
		if err != nil {
			return sum, err
		}
		sum.Attachments += n
	}
	return sum, nil
}

// copyAttachments copies the files attached to entity id.
func copyAttachments(ctx context.Context, src, dst store.AttachmentManager, id string) (int, error) {
	infos, err := src.ListFamilyAttachments(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		// The source has no face of id, so nothing is attached to it.
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("list attachments of %s: %w", id, err)
	}
	for _, info := range infos {
		if err := copyAttachment(ctx, src, dst, info); err != nil {
			return 0, err
		}
	}
	return len(infos), nil
}

func copyAttachment(ctx context.Context, src, dst store.AttachmentManager, info store.AttachmentInfo) error {
	r, err := src.ReadFamilyAttachment(ctx, info.EntityID, info.Property, info.FileName)
	if err != nil {
		return fmt.Errorf("read attachment %s/%s/%s: %w", info.EntityID, info.Property, info.FileName, err)
	}
	defer func() { _ = r.Close() }()
	if err := dst.AttachFamilyFile(ctx, info.EntityID, info.Property, info.FileName, r); err != nil {
		return fmt.Errorf("write attachment %s/%s/%s: %w", info.EntityID, info.Property, info.FileName, err)
	}
	return nil
}
