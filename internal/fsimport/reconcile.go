package fsimport

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/frontmatter"
)

// attachTempPrefix marks fsstore's in-progress attachment files
// (fsstore.attachTempPrefix).
const attachTempPrefix = ".rela-attach-tmp-"

// reconcileFiles walks the source's data folders and accounts for every
// file: it was imported, it is listed as not copied with a reason, or it is an
// error.
//
// fsstore skips what it cannot read (undeclared type folders, file names
// that are not ids) and lets a later file shadow an earlier one with the same
// id. Without this walk those files would vanish from the target without a
// word, and the fs project would look fully imported.
func reconcileFiles(src *source, imported importedSet, rep *Report) error {
	root, err := os.OpenRoot(src.root)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()

	pending, err := readPendingDeletes(root)
	if err != nil {
		rep.fail("read .rela/pending-deletes.json: %v", err)
	}
	rep.PendingDeletes = sortedKeys(pending)

	w := &fileWalk{src: src, root: root, imported: imported, pending: pending, rep: rep}
	for _, step := range []func() error{w.entities, w.relations, w.attachments} {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}

type fileWalk struct {
	src      *source
	root     *os.Root
	imported importedSet
	pending  map[string]bool
	rep      *Report
}

// walk visits the files below dir, which may be absent.
func (w *fileWalk) walk(dir string, visit func(p string, d fs.DirEntry)) error {
	if _, err := w.root.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return fs.WalkDir(w.root.FS(), dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			visit(p, d)
		}
		return nil
	})
}

func (w *fileWalk) entities() error {
	pluralToType := map[string]string{}
	for typ, def := range w.src.meta.Entities {
		pluralToType[def.GetPlural(typ)] = typ
	}
	return w.walk("entities", func(p string, _ fs.DirEntry) {
		parts := strings.Split(p, "/")
		if len(parts) != 3 {
			w.rep.skip(p, "not inside an entity type folder")
			return
		}
		typ, ok := pluralToType[parts[1]]
		if !ok {
			w.rep.skip(p, "folder %q is not a type the schema declares", parts[1])
			return
		}
		stem, isMD := strings.CutSuffix(parts[2], ".md")
		if !isMD {
			w.rep.skip(p, "not a markdown file")
			return
		}
		id, _, err := entity.ParseStateRef(stem)
		if err != nil {
			w.rep.skip(p, "the file name is not a valid entity id")
			return
		}
		read := w.imported.readEntities[stem]
		switch got, ok := w.imported.entities[stem]; {
		case ok && got == typ:
			w.checkFrontmatterID(p, id)
		case len(read) > 1:
			w.rep.fail("%s: id %s is in more than one type folder; "+
				"the same id in two type folders is ambiguous", p, stem)
		case len(read) == 1 && read[0] == typ:
			// Read but not written; the write error is already reported.
		case w.pending[id]:
			w.rep.skip(p, "deleted in the source (undo still pending)")
		case len(read) == 1:
			w.rep.fail("%s: id %s also exists as a %s; the same id in two type folders is ambiguous", p, stem, read[0])
		default:
			w.rep.fail("%s: the source store did not read this file", p)
		}
	})
}

// checkFrontmatterID warns when the frontmatter names a different id than
// the file. The file name wins, as it does in fsstore, but the author may
// have meant the other one.
func (w *fileWalk) checkFrontmatterID(p, id string) {
	data, err := w.root.ReadFile(p)
	if err != nil {
		return
	}
	fm, _ := frontmatter.Split(string(data))
	var head struct {
		ID string `yaml:"id"`
	}
	if yaml.Unmarshal([]byte(fm), &head) != nil || head.ID == "" || head.ID == id {
		return
	}
	w.rep.warn("%s: the frontmatter says id %q; imported as %s, the file name", p, head.ID, id)
}

func (w *fileWalk) relations() error {
	return w.walk("relations", func(p string, _ fs.DirEntry) {
		name := strings.TrimPrefix(p, "relations/")
		stem, isMD := strings.CutSuffix(name, ".md")
		switch {
		case strings.Contains(name, "/"):
			w.rep.skip(p, "not directly inside relations/")
			return
		case !isMD:
			w.rep.skip(p, "not a markdown file")
			return
		case w.imported.relations[stem], w.imported.readRelations[stem]:
			// Imported, or read and its write error already reported.
			return
		}
		from, typ, to := splitRelationStem(stem)
		if typ == "" {
			w.rep.skip(p, "the file name is not FROM--TYPE--TO")
			return
		}
		fromID, _, err := entity.ParseStateRef(from)
		if err != nil {
			w.rep.skip(p, "the file name is not FROM--TYPE--TO")
			return
		}
		if w.pending[fromID] || w.pending[to] {
			w.rep.skip(p, "an endpoint was deleted in the source (undo still pending)")
			return
		}
		w.rep.fail("%s: %s", p, w.relationProblem(p, stem))
	})
}

// relationProblem says why a relation file was not imported. fsstore keys a
// relation by its file name but reads its identity from the frontmatter, so
// the two disagreeing, or a missing key, is the usual cause.
func (w *fileWalk) relationProblem(p, stem string) string {
	data, err := w.root.ReadFile(p)
	if err != nil {
		return "the source store did not read this relation: " + err.Error()
	}
	fm, _ := frontmatter.Split(string(data))
	var head struct {
		From     string `yaml:"from"`
		FromFace string `yaml:"from_face"`
		Relation string `yaml:"relation"`
		To       string `yaml:"to"`
	}
	if err := yaml.Unmarshal([]byte(fm), &head); err != nil {
		return "the frontmatter is not valid YAML: " + err.Error()
	}
	var missing []string
	for _, k := range []struct{ key, val string }{
		{"from", head.From}, {"relation", head.Relation}, {"to", head.To},
	} {
		if k.val == "" {
			missing = append(missing, k.key+":")
		}
	}
	if len(missing) > 0 {
		return "the frontmatter has no " + strings.Join(missing, ", ") +
			" (a relation file names its type with relation:, not type:)"
	}
	named := head.From
	if head.FromFace != "" {
		named += entity.StateRefSeparator + head.FromFace
	}
	named += "--" + head.Relation + "--" + head.To
	if named != stem {
		return "the frontmatter names " + named + ", which disagrees with the file name"
	}
	return "the source store did not read this relation"
}

func (w *fileWalk) attachments() error {
	return w.walk("attachments", func(p string, _ fs.DirEntry) {
		rel := strings.TrimPrefix(p, "attachments/")
		parts := strings.Split(rel, "/")
		switch {
		case len(parts) != 3:
			w.rep.skip(p, "not an attachment path (attachments/ID/PROPERTY/FILE)")
		case strings.HasPrefix(parts[2], attachTempPrefix):
			w.rep.skip(p, "unfinished upload")
		case w.imported.attach[rel]:
		case w.pending[parts[0]]:
			w.rep.skip(p, "its entity was deleted in the source (undo still pending)")
		case w.imported.families[parts[0]] == "":
			w.rep.skip(p, "its entity is not in the source")
		default:
			w.rep.fail("%s: the source store did not read this attachment", p)
		}
	})
}

// splitRelationStem splits "FROM--TYPE--TO" the way fsstore does
// (fsstore.parseRelationFilename): TYPE is between the first and the last
// "--". Empty results mean the stem is not a relation name.
func splitRelationStem(stem string) (from, typ, to string) {
	i := strings.Index(stem, "--")
	if i < 1 {
		return "", "", ""
	}
	rest := stem[i+2:]
	j := strings.LastIndex(rest, "--")
	if j < 1 || j+2 == len(rest) {
		return "", "", ""
	}
	return stem[:i], rest[:j], rest[j+2:]
}

func readPendingDeletes(root *os.Root) (map[string]bool, error) {
	out := map[string]bool{}
	data, err := root.ReadFile(".rela/pending-deletes.json")
	if errors.Is(err, fs.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	var entries []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		return out, err
	}
	for _, e := range entries {
		out[e.ID] = true
	}
	return out, nil
}
