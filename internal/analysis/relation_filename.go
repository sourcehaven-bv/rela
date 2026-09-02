package analysis

import (
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/frontmatter"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// RelationFilenameReason classifies a [RelationFilenameIssue]. A closed set:
// the CLI switches on it to choose a rendering, so a new value must be added
// here and handled there rather than inferred from which fields happen to be
// empty.
type RelationFilenameReason string

const (
	// ReasonMismatch: the filename and the frontmatter describe different
	// relations. The store indexed the FILENAME triple, so the relation the
	// content describes does not exist in the graph.
	ReasonMismatch RelationFilenameReason = "filename and content describe different relations"

	// ReasonUnparseableName: the filename is not FROM--TYPE--TO, so the store
	// skipped it entirely — the relation is not in the graph at all.
	ReasonUnparseableName RelationFilenameReason = "filename is not FROM--TYPE--TO"

	// ReasonLegacyTypeKey: the frontmatter spells the relation type `type:`
	// instead of `relation:`. The store reads only `relation:`, so the
	// relation loads with an EMPTY type while the index (built from the
	// filename) says otherwise — the two disagree, silently.
	ReasonLegacyTypeKey RelationFilenameReason = "legacy `type:` key; the store reads `relation:` and loads this relation with an empty type"
)

// RelationFilenameIssue reports a relation file whose NAME disagrees with its
// CONTENT — the filename parses to one (from, type, to) triple while the
// frontmatter declares another.
type RelationFilenameIssue struct {
	// File is the path as an operator would open it.
	File string
	// FromFilename is the triple the filename encodes — the one the store
	// actually indexed the relation under.
	FromFilename string
	// FromContent is the triple the frontmatter declares — the one the author
	// meant, and the one every other tool reports as missing.
	FromContent string
	// Reason classifies the finding. See the Reason* constants.
	Reason RelationFilenameReason
}

// CheckRelationFilenames reports relation files whose filename and content
// disagree about which relation they are.
//
// # Why this exists as a check rather than a fix
//
// fsstore keys relations ENTIRELY on the filename: syncRelations parses
// `FROM--TYPE--TO.md` and never opens the file to confirm the triple. A file
// whose name and content disagree is therefore indexed under the FILENAME
// triple, and the relation its content describes does not exist as far as the
// graph is concerned.
//
// The symptom an operator actually sees is a cardinality error naming the
// VICTIM entity ("must have at least 1 X relation(s), has 0") with no path back
// to the malformed file — which is exactly how issue #1004 was reported, after
// the reporter had lost time to it. This check turns that into a finding that
// names the file.
//
// The rename path that most plausibly produced such files is already correct:
// fsstore.renameEntity rewrites each incident relation under its new filename
// and removes the old one. So this detects LEGACY corruption and hand-edits,
// not an ongoing defect.
//
// Returns nil when the service was built without FS + Paths — same gate as
// [Service.FindOrphanedTempFiles]; analyses that don't touch the filesystem
// still work.
func (s *Service) CheckRelationFilenames() []RelationFilenameIssue {
	if s.deps.FS == nil || s.deps.Paths == nil {
		return nil
	}

	entries, err := s.deps.FS.ReadDir(s.deps.Paths.RelationsDir)
	if err != nil {
		// A missing relations/ directory is not a finding — a project may
		// legitimately have no relations yet.
		return nil
	}

	issues := make([]RelationFilenameIssue, 0)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(s.deps.Paths.RelationsDir, e.Name())
		if iss := checkRelationFile(s.deps.FS, path, e.Name()); iss != nil {
			issues = append(issues, *iss)
		}
	}

	sort.Slice(issues, func(i, j int) bool { return issues[i].File < issues[j].File })
	return issues
}

// checkRelationFile compares one file's name against its frontmatter.
// Returns nil when they agree, or when the file cannot be read or parsed —
// an unreadable file is a different problem (a permission error, or a
// git-crypt-encrypted relation, which fsstore handles as a locked shell) and
// reporting it here would put a second, unrelated class of finding into a
// check operators run to answer one question.
func checkRelationFile(fs storage.FS, path, name string) *RelationFilenameIssue {
	base := strings.TrimSuffix(name, ".md")
	nameFrom, nameType, nameTo := splitRelationFilename(base)
	if nameFrom == "" {
		return &RelationFilenameIssue{
			File:         path,
			FromFilename: base,
			Reason:       ReasonUnparseableName,
		}
	}
	nameTriple := nameFrom + "--" + nameType + "--" + nameTo

	raw, err := fs.ReadFile(path)
	if err != nil {
		return nil
	}
	// The SAME splitter the store uses (internal/frontmatter is the shared
	// leaf that markdown and fsstore both build on), then plain yaml. Reading
	// the file a third way would make any divergence a false negative in a
	// corruption detector.
	fmBlock, _ := frontmatter.Split(string(raw))
	fm := map[string]string{}
	if err := yaml.Unmarshal([]byte(fmBlock), &fm); err != nil {
		return nil
	}

	cFrom := fm["from"]
	cTo := fm["to"]
	cType, hasRelation := fm["relation"]

	// `relation:` is what the store reads and what every write emits. `type:`
	// is a legacy spelling still present in older files, and it does NOT work:
	// mdCodec builds the relation from doc.getString("relation"), so a legacy
	// file loads with an EMPTY type while the index — built from the filename —
	// says otherwise. `type` is not a reserved relation key either, so it is
	// additionally stored as a user property.
	//
	// That is the same filename-vs-content divergence this check exists for,
	// and it is worse than the #1004 shape: #1004 fails loudly downstream as a
	// cardinality error, this one is silently inconsistent, and a read →
	// write round trip through any path (formatter, rename, migration) writes
	// the empty type back and destroys the last record of it.
	if !hasRelation {
		if legacy, ok := fm["type"]; ok && legacy != "" {
			return &RelationFilenameIssue{
				File:         path,
				FromFilename: nameTriple,
				FromContent:  cFrom + "--" + legacy + "--" + cTo,
				Reason:       ReasonLegacyTypeKey,
			}
		}
	}

	// Frontmatter that declares none of the three is a different shape of
	// broken (an empty or non-relation file); not this check's business.
	if cFrom == "" && cType == "" && cTo == "" {
		return nil
	}

	if cFrom == nameFrom && cType == nameType && cTo == nameTo {
		return nil
	}
	return &RelationFilenameIssue{
		File:         path,
		FromFilename: nameTriple,
		FromContent:  cFrom + "--" + cType + "--" + cTo,
		Reason:       ReasonMismatch,
	}
}

// splitRelationFilename splits "FROM--TYPE--TO" into its three parts.
//
// MUST stay behaviorally identical to fsstore.parseRelationFilename
// (internal/store/fsstore/index.go). Duplicated rather than shared because it
// is unexported there and arch-lint forbids analysis -> store/fsstore, which is
// the right boundary — a storage detail should not become an analysis
// dependency to save nine lines.
//
// The exactness is the point, not an accident: this check's entire claim is
// "I split names the way the indexer does". A divergence here reports a false
// mismatch on a perfectly good file whose relation type contains "--".
// TestSplitRelationFilename_MatchesIndexer pins the agreement.
//
// First "--" ends FROM, last "--" starts TO.
func splitRelationFilename(name string) (from, relType, to string) {
	i := strings.Index(name, "--")
	if i < 1 {
		return "", "", ""
	}
	from = name[:i]
	rest := name[i+2:]

	j := strings.LastIndex(rest, "--")
	if j < 1 {
		return "", "", ""
	}
	relType = rest[:j]
	to = rest[j+2:]
	if to == "" {
		return "", "", ""
	}
	return from, relType, to
}
