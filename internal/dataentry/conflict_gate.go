package dataentry

import (
	"context"
	"log/slog"
	"net/http"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/conflict"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
)

// The conflict routes read files the store cannot parse, so they cannot use
// the resolver's row read. They ask the same row gate about the identities a
// file names instead (BUG-Q3Z15V, BUG-0E6S7Q). Where a file sits decides
// what it is, as it does for the file store:
//
//   - An entity file sits at <entities>/<type dir>/<id>[@face].md. It names
//     the row of its file name and directory, and the id and type on each
//     parsed side. Every one of them must be readable, at the face the file
//     name gives.
//   - A relation file sits at <relations>/<from>[@face]--<type>--<to>.md. It
//     names the edge of its file name and the edge on each parsed side. The
//     endpoints of every one must be readable, as for a relation read.
//
// A file the gate cannot judge is unreadable: one outside those two
// directories, one whose directory maps to no type, one whose name does not
// parse, one with a side whose identity is missing, or one with a side the
// parser read as the other kind than its directory says. Hidden files are left
// out of the list and answer the uniform 404 on the detail and the resolve,
// the same answer as a missing file.

// conflictIdentity is what one conflicted file names, as the gate judges it.
// At most one of rows and rels is set; neither means the gate cannot judge
// the file.
type conflictIdentity struct {
	// file is the row an entity file's path names. Its face is also the face
	// the redaction judges each side at.
	file entityPkg.Entity
	// edge is the edge a relation file's name names.
	edge entityPkg.Relation
	rows []*entityPkg.Entity
	rels []*entityPkg.Relation
}

// face is the face the file's sides are judged at: the entity file's face,
// or the relation file's tail face.
func (id conflictIdentity) face() entityPkg.Face {
	if len(id.rels) > 0 {
		return id.edge.FromFace
	}
	return id.file.Face
}

// identifyConflict derives the identities cf names. paths locates the
// entity and relation directories; meta maps an entity directory to its
// type.
func identifyConflict(paths *project.Context, meta *metamodel.Metamodel, cf *conflict.ConflictedFile) conflictIdentity {
	if rel, ok := pathUnder(paths.RelationsDir, cf.Path); ok && !strings.ContainsRune(rel, filepath.Separator) {
		return identifyRelationConflict(strings.TrimSuffix(rel, ".md"), cf)
	}
	rel, ok := pathUnder(paths.EntitiesDir, cf.Path)
	if !ok {
		return conflictIdentity{}
	}
	dir, name, ok := strings.Cut(rel, string(filepath.Separator))
	if !ok || strings.ContainsRune(name, filepath.Separator) {
		return conflictIdentity{}
	}
	typ := entityTypeOfDir(meta, dir)
	fileID, face, err := entityPkg.ParseStateRef(strings.TrimSuffix(name, ".md"))
	if typ == "" || err != nil {
		return conflictIdentity{}
	}
	out := conflictIdentity{file: entityPkg.Entity{ID: fileID, Type: typ, Face: face}}
	out.rows = append(out.rows, &entityPkg.Entity{ID: fileID, Type: typ, Face: face})
	for _, side := range conflictSides(cf) {
		e := side.Entity
		switch {
		case side.Relation != nil:
			// The parser read a relation where the directory says entity
			// (a file named like FROM--type--TO.md). Judging it on either
			// branch would skip the other's redaction.
			return conflictIdentity{}
		case e == nil:
		case e.ID != "" && e.Type != "":
			out.rows = append(out.rows, &entityPkg.Entity{ID: e.ID, Type: e.Type, Face: face})
		case e.ID == "" && len(e.Properties) == 0:
			// An empty side: the file was deleted on that branch.
		default:
			// A side that serves values under an identity the gate cannot
			// ask about.
			return conflictIdentity{}
		}
	}
	return out
}

// identifyRelationConflict is [identifyConflict] for a relation file, whose
// name stem is stem. Every edge carries the tail face of the file name,
// because a side's frontmatter names only the bare id.
func identifyRelationConflict(stem string, cf *conflict.ConflictedFile) conflictIdentity {
	parts := strings.Split(stem, "--")
	if len(parts) != 3 {
		return conflictIdentity{}
	}
	from, face, err := entityPkg.ParseStateRef(parts[0])
	if err != nil {
		return conflictIdentity{}
	}
	out := conflictIdentity{edge: entityPkg.Relation{From: from, FromFace: face, Type: parts[1], To: parts[2]}}
	edge := out.edge
	out.rels = append(out.rels, &edge)
	for _, side := range conflictSides(cf) {
		if side.Entity != nil {
			// The parser read an entity where the directory says relation.
			return conflictIdentity{}
		}
		if side.Relation == nil {
			continue
		}
		rel := *side.Relation
		rel.FromFace = face
		out.rels = append(out.rels, &rel)
	}
	return out
}

// conflictSides returns cf's parsed sides that are present.
func conflictSides(cf *conflict.ConflictedFile) []*conflict.ParsedSide {
	var out []*conflict.ParsedSide
	for _, side := range []*conflict.ParsedSide{cf.Ours, cf.Theirs} {
		if side != nil {
			out = append(out, side)
		}
	}
	return out
}

// pathUnder returns path relative to dir when path lies inside dir. Both are
// compared with symlinks resolved, as the caller-supplied conflict path is.
// An empty dir contains nothing.
func pathUnder(dir, path string) (string, bool) {
	if dir == "" {
		return "", false
	}
	rel, err := filepath.Rel(resolvedPath(dir), resolvedPath(path))
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return rel, true
}

// resolvedPath is p made absolute with symlinks resolved. For a path that
// does not exist, such as a file a branch deleted, it resolves the nearest
// parent that does, so the path still compares inside its directory.
func resolvedPath(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		return resolved
	}
	parent := filepath.Dir(p)
	if parent == p {
		return p
	}
	return filepath.Join(resolvedPath(parent), filepath.Base(p))
}

// entityTypeOfDir maps an entity directory name to its type: a type's own
// name first, then its plural, as the file store lays it out. Names are
// tried in sorted order so the answer does not depend on map order.
func entityTypeOfDir(meta *metamodel.Metamodel, dir string) string {
	if meta == nil {
		return ""
	}
	if _, ok := meta.Entities[dir]; ok {
		return dir
	}
	names := make([]string, 0, len(meta.Entities))
	for name := range meta.Entities {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		def := meta.Entities[name]
		if dir == def.GetPlural(name) {
			return name
		}
	}
	return ""
}

// conflictsReadable reports, per identity, whether the principal may read
// everything it names. It asks one gate query per entity type and one
// endpoint batch for every relation, however many files there are. The row
// gate is the one vr's resolver is built over. A gate failure is returned,
// never folded into "hidden".
func conflictsReadable(ctx context.Context, vr visibleReader, ids []conflictIdentity) ([]bool, error) {
	byType := map[string][]string{}
	var rels []*entityPkg.Relation
	for _, id := range ids {
		for _, row := range id.rows {
			byType[row.Type] = append(byType[row.Type], row.ID)
		}
		rels = append(rels, id.rels...)
	}
	verdicts := make(map[string]acl.FaceVerdicts, len(byType))
	for typ, typeIDs := range byType {
		vs, err := ctxRowGate{}.ReadableFacesMany(ctx, typ, typeIDs)
		if err != nil {
			return nil, err
		}
		verdicts[typ] = vs
	}
	var relOK []bool
	if len(rels) > 0 {
		var err error
		relOK, err = vr.resolver.EndpointsReadableErr(ctx, rels)
		if err != nil {
			return nil, err
		}
	}

	out := make([]bool, len(ids))
	next := 0
	for i, id := range ids {
		ok := len(id.rows) > 0 || len(id.rels) > 0
		for _, row := range id.rows {
			ok = ok && verdicts[row.Type].For(row.ID).Contains(row.Face)
		}
		for range id.rels {
			ok = ok && relOK[next]
			next++
		}
		out[i] = ok
	}
	return out, nil
}

// conflictConfigFiles are the project-root config files whose conflict path
// is listed to every caller of the status and the sync. The configuration is
// not a secret, so naming one discloses nothing.
var conflictConfigFiles = map[string]bool{
	project.SchemaFile: true, project.LegacySchemaFile: true,
	"data-entry.yaml": true, "desktop.yaml": true, "acl.yaml": true, "schedules.yaml": true,
	"subscriptions.yaml": true, "mail-templates.yaml": true, "classification.yaml": true,
	"realtime.yaml": true, "flows.yaml": true,
}

// conflictConfigDirs are the project directories that hold only config, for
// the same list.
var conflictConfigDirs = map[string]bool{
	"scripts": true, "lib": true, "actions": true, "validations": true, "migrations": true,
	project.TemplatesDir: true, project.CustomDir: true, project.AppsDir: true,
}

// readableConflictPaths returns the paths in relPaths the principal may see,
// in order. The paths are relative to the project root, as git reports them.
// It is the gate of the conflict list for a surface that names conflict
// paths without listing conflicts (BUG-197XPAU).
//
// It fails closed. A path is kept only when it is one of these:
//
//   - A file under the entity or relation directory whose identity the
//     principal may read. A data file that does not parse as conflicted,
//     such as one a branch deleted, is judged by the row its path names.
//   - A file under <attachments>/<id>/ when the principal may read some face
//     of id. The attachment of a deleted entity is left out.
//   - A config file in [conflictConfigFiles] or under [conflictConfigDirs].
//
// Every other path is left out, because it may name an entity whose
// existence is secret.
func readableConflictPaths(
	ctx context.Context, vr visibleReader, paths *project.Context, meta *metamodel.Metamodel, relPaths []string,
) ([]string, error) {
	keep := make([]bool, len(relPaths))
	gated := make([]int, 0, len(relPaths))
	ids := make([]conflictIdentity, 0, len(relPaths))
	attachOwners := map[int]string{}
	attachDir := filepath.Join(paths.Root, project.AttachmentsDir)
	for i, rel := range relPaths {
		abs := filepath.Join(paths.Root, filepath.FromSlash(rel))
		_, inEntities := pathUnder(paths.EntitiesDir, abs)
		_, inRelations := pathUnder(paths.RelationsDir, abs)
		if inEntities || inRelations {
			cf, err := conflict.ParseConflictedFile(abs, meta)
			if err != nil {
				cf = &conflict.ConflictedFile{Path: abs}
			}
			gated = append(gated, i)
			ids = append(ids, identifyConflict(paths, meta, cf))
			continue
		}
		if under, ok := pathUnder(attachDir, abs); ok {
			if owner, _, nested := strings.Cut(under, string(filepath.Separator)); nested && owner != "" {
				attachOwners[i] = owner
			}
			continue
		}
		keep[i] = isConflictConfigPath(paths.Root, abs)
	}
	readable, err := conflictsReadable(ctx, vr, ids)
	if err != nil {
		return nil, err
	}
	for j, i := range gated {
		keep[i] = readable[j]
	}
	if err := keepReadableAttachments(ctx, vr, attachOwners, keep); err != nil {
		return nil, err
	}
	// Nil, not empty, when nothing is left, so an omitempty field drops it.
	var out []string
	for i, rel := range relPaths {
		if keep[i] {
			out = append(out, rel)
		}
	}
	return out, nil
}

// keepReadableAttachments sets keep[i] for each attachment path i whose
// owning entity id the principal may read at some face. It asks one batch
// for every owner.
func keepReadableAttachments(ctx context.Context, vr visibleReader, owners map[int]string, keep []bool) error {
	if len(owners) == 0 {
		return nil
	}
	ids := make([]string, 0, len(owners))
	for _, id := range owners {
		ids = append(ids, id)
	}
	types, err := vr.readableTypes(ctx, ids)
	if err != nil {
		return err
	}
	for i, id := range owners {
		keep[i] = types[id] != ""
	}
	return nil
}

// isConflictConfigPath reports whether abs, under root, is a config file
// whose conflict path any caller may see.
func isConflictConfigPath(root, abs string) bool {
	rel, ok := pathUnder(root, abs)
	if !ok {
		return false
	}
	first, _, nested := strings.Cut(rel, string(filepath.Separator))
	if nested {
		return conflictConfigDirs[first]
	}
	return conflictConfigFiles[first]
}

// conflictReadableOr404 gates one conflicted file's identity. It writes the
// uniform not-found when the principal may not read it, or the gate error.
//
//archplan:guard=authz reason="row gate for a conflicted file: a file naming a hidden row is the uniform 404"
func conflictReadableOr404(
	w http.ResponseWriter, r *http.Request, vr visibleReader, id conflictIdentity,
) bool {
	ok, err := conflictsReadable(r.Context(), vr, []conflictIdentity{id})
	if err != nil {
		writeGateError(w, r, err)
		return false
	}
	if !ok[0] {
		writeConflictNotFound(w, r)
		return false
	}
	return true
}

// parseConflictOr404 parses the conflicted file at absPath. A file that
// cannot be read or holds no conflict markers answers the uniform 404,
// before any gate runs: a 500 there would tell a caller which hidden
// entities have a file (BUG-Q3Z15V).
func parseConflictOr404(
	w http.ResponseWriter, r *http.Request, absPath string, meta *metamodel.Metamodel,
) (*conflict.ConflictedFile, bool) {
	cf, err := conflict.ParseConflictedFile(absPath, meta)
	if err != nil {
		slog.Debug("dataentry: conflict file did not parse as conflicted", "path", absPath, "err", err)
		writeConflictNotFound(w, r)
		return nil, false
	}
	return cf, true
}

// writeConflictNotFound is the one answer for a conflicted file that is
// missing, one that is not conflicted, and one the principal may not read.
func writeConflictNotFound(w http.ResponseWriter, r *http.Request) {
	writeV1Error(w, r, http.StatusNotFound, "conflict_not_found", "Conflicted file not found", "")
}

// conflictHiddenProperties returns the properties `visible:` hides on either
// side of cf, so the detail serves neither side's value of them. An entity
// side is judged as the row at face; a relation side as the edge from its
// source row, as the relation read judges its meta.
func (svc affordanceService) conflictHiddenProperties(
	ctx context.Context, cf *conflict.ConflictedFile, face entityPkg.Face,
) map[string]struct{} {
	hidden := map[string]struct{}{}
	for _, side := range conflictSides(cf) {
		switch {
		case side.Entity != nil:
			row := *side.Entity
			row.Face = face
			for name := range svc.hiddenProperties(ctx, &row) {
				hidden[name] = struct{}{}
			}
		case side.Relation != nil:
			rel := side.Relation
			visible := svc.visibleRelationMetaIncoming(ctx,
				entityPkg.Ref{ID: rel.From, Face: face}, rel.Type, rel.Properties)
			for name := range rel.Properties {
				if _, ok := visible[name]; !ok {
					hidden[name] = struct{}{}
				}
			}
		}
	}
	return hidden
}
