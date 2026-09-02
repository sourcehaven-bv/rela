package analysis_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/analysis"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
)

// newFSService builds an analysis.Service with a real (in-memory) filesystem,
// which CheckRelationFilenames needs — the graph store cannot see this problem,
// since a mis-filed relation is indexed under the wrong key and looks like an
// ordinary relation between entities that may not exist.
func newFSService(t *testing.T, files map[string]string) *analysis.Service {
	t.Helper()
	fs := storage.NewMemFS()
	paths := &project.Context{Root: "/p", RelationsDir: "/p/relations"}
	if err := fs.MkdirAll(paths.RelationsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for name, body := range files {
		if err := fs.WriteFile(paths.RelationsDir+"/"+name, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	st := memstore.New()
	meta := &metamodel.Metamodel{Entities: map[string]metamodel.EntityDef{}}
	tr := tracer.New(st)
	svc, err := analysis.New(analysis.Deps{
		Store: st, Meta: meta, Tracer: tr, FS: fs, Paths: paths,
		LuaReadDeps: lua.ReadDeps{VisibleReader: st, Tracer: tr, Meta: meta},
	})
	if err != nil {
		t.Fatalf("analysis.New: %v", err)
	}
	return svc
}

func relFile(from, relType, to string) string {
	return "---\nfrom: " + from + "\nrelation: " + relType + "\nto: " + to + "\n---\n"
}

// TestCheckRelationFilenames_ReportersScenario reproduces the exact corruption
// from issue #1004 and asserts the check names the FILE — which is what the
// reporter could not get from any existing output.
//
// The symptom they saw was a cardinality error naming PRS-FUNC-0Q7E (the entity
// the content correctly references) with no route back to the file, because the
// store had indexed the relation under PRS-FUNC-8Q7E (the filename) and
// PRS-FUNC-8Q7E does not exist.
func TestCheckRelationFilenames_ReportersScenario(t *testing.T) {
	t.Parallel()

	svc := newFSService(t, map[string]string{
		"PRS-FLOW-1HL6--wordtUitgevoerdDoor--PRS-FUNC-8Q7E.md": relFile(
			"PRS-FLOW-1HL6", "wordtUitgevoerdDoor", "PRS-FUNC-0Q7E"),
	})

	issues := svc.CheckRelationFilenames()
	if len(issues) != 1 {
		t.Fatalf("want 1 issue, got %d: %+v", len(issues), issues)
	}
	got := issues[0]
	if got.File != "/p/relations/PRS-FLOW-1HL6--wordtUitgevoerdDoor--PRS-FUNC-8Q7E.md" {
		t.Errorf("File must name the malformed file, got %q", got.File)
	}
	// Both triples must be reported: the filename one is what the store
	// indexed, the content one is what every other tool says is missing.
	// Only showing one leaves the operator to guess the mapping.
	if got.FromFilename != "PRS-FLOW-1HL6--wordtUitgevoerdDoor--PRS-FUNC-8Q7E" {
		t.Errorf("FromFilename = %q", got.FromFilename)
	}
	if got.FromContent != "PRS-FLOW-1HL6--wordtUitgevoerdDoor--PRS-FUNC-0Q7E" {
		t.Errorf("FromContent = %q", got.FromContent)
	}
}

func TestCheckRelationFilenames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		files     map[string]string
		wantCount int
		wantWhy   analysis.RelationFilenameReason
	}{
		{
			name:      "consistent file is not flagged",
			files:     map[string]string{"A--rel--B.md": relFile("A", "rel", "B")},
			wantCount: 0,
		},
		{
			name:      "mismatched to",
			files:     map[string]string{"A--rel--B.md": relFile("A", "rel", "C")},
			wantCount: 1,
			wantWhy:   analysis.ReasonMismatch,
		},
		{
			name:      "mismatched from",
			files:     map[string]string{"A--rel--B.md": relFile("Z", "rel", "B")},
			wantCount: 1,
			wantWhy:   analysis.ReasonMismatch,
		},
		{
			name:      "mismatched type",
			files:     map[string]string{"A--rel--B.md": relFile("A", "other", "B")},
			wantCount: 1,
			wantWhy:   analysis.ReasonMismatch,
		},
		{
			name:      "unparseable filename",
			files:     map[string]string{"notatriple.md": relFile("A", "rel", "B")},
			wantCount: 1,
			wantWhy:   analysis.ReasonUnparseableName,
		},
		{
			// A relation type containing "--" still round-trips, because the
			// split takes the FIRST "--" for from and the LAST for to — the
			// same rule fsstore's indexer uses. If this check split
			// differently it would report false positives on valid files.
			name:      "relation type containing the separator",
			files:     map[string]string{"A--we--ird--B.md": relFile("A", "we--ird", "B")},
			wantCount: 0,
		},
		{
			// Not this check's business: an empty or non-relation file is a
			// different problem with its own reporting, and flagging it here
			// would drown the signal.
			name:      "file with no relation frontmatter is ignored",
			files:     map[string]string{"A--rel--B.md": "---\ntitle: not a relation\n---\n"},
			wantCount: 0,
		},
		{
			// `type:` is a legacy spelling that does NOT work. mdCodec builds
			// the relation from doc.getString("relation"), so such a file
			// loads with an EMPTY type while the index — built from the
			// filename — says otherwise. That is this check's subject, and a
			// worse variant of it than #1004: #1004 fails loudly downstream
			// as a cardinality error, this one is silently inconsistent.
			//
			// An earlier version of this check treated these as false
			// positives and suppressed them. They were 37 TRUE positives in
			// this repo's own tickets project, where three relations on
			// BUG-2OXEW0 rendered with a blank type. Reported separately so
			// they are triageable without drowning the mismatch findings.
			name:      "legacy type key is reported",
			files:     map[string]string{"A--rel--B.md": "---\nfrom: A\ntype: rel\nto: B\n---\n"},
			wantCount: 1,
			wantWhy:   analysis.ReasonLegacyTypeKey,
		},
		{
			// `relation:` present wins outright — a file carrying both keys
			// loads fine, because that is the key the store reads.
			name: "both keys present is not a finding",
			files: map[string]string{
				"A--rel--B.md": "---\nfrom: A\nrelation: rel\ntype: rel\nto: B\n---\n",
			},
			wantCount: 0,
		},
		{
			name:      "non-markdown file is ignored",
			files:     map[string]string{"README.txt": "whatever"},
			wantCount: 0,
		},
		{
			// Parsing goes through internal/frontmatter + yaml — the same
			// splitter the store uses — rather than a private scanner, so
			// these shapes are handled by the shared code rather than by a
			// third reading of the file. Kept as end-to-end cases because a
			// misparse here becomes a FALSE FINDING on a good file, which is
			// how a corruption detector loses its readers.
			name:      "CRLF line endings",
			files:     map[string]string{"A--rel--B.md": "---\r\nfrom: A\r\nrelation: rel\r\nto: B\r\n---\r\n"},
			wantCount: 0,
		},
		{
			name:      "quoted values",
			files:     map[string]string{"A--rel--B.md": "---\nfrom: \"A\"\nrelation: 'rel'\nto: B\n---\n"},
			wantCount: 0,
		},
		{
			name:      "--- inside the body does not leak keys",
			files:     map[string]string{"A--rel--B.md": "---\nfrom: A\nrelation: rel\nto: B\n---\nbody\n---\nfrom: EVIL\n"},
			wantCount: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			issues := newFSService(t, tc.files).CheckRelationFilenames()
			if len(issues) != tc.wantCount {
				t.Fatalf("want %d issue(s), got %d: %+v", tc.wantCount, len(issues), issues)
			}
			if tc.wantCount > 0 && issues[0].Reason != tc.wantWhy {
				t.Errorf("Reason = %q, want %q", issues[0].Reason, tc.wantWhy)
			}
		})
	}
}

// TestCheckRelationFilenames_NoFS pins the optional-dependency gate: a service
// built without FS + Paths returns nil rather than panicking, matching
// FindOrphanedTempFiles.
func TestCheckRelationFilenames_NoFS(t *testing.T) {
	t.Parallel()

	st := memstore.New()
	meta := &metamodel.Metamodel{Entities: map[string]metamodel.EntityDef{}}
	tr := tracer.New(st)
	svc, err := analysis.New(analysis.Deps{
		Store: st, Meta: meta, Tracer: tr,
		LuaReadDeps: lua.ReadDeps{VisibleReader: st, Tracer: tr, Meta: meta},
	})
	if err != nil {
		t.Fatalf("analysis.New: %v", err)
	}
	if got := svc.CheckRelationFilenames(); got != nil {
		t.Errorf("want nil without FS/Paths, got %+v", got)
	}
}
