package lua

import (
	"bytes"
	"context"
	"iter"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TKT-ACSBSA: admin.get_entity / list_entities / get_relations inside a
// rela.bypass_acl closure read RAW — past the gate the surrounding rela.*
// bindings enforce.
//
// The pivot of every test here is that the runtime's VisibleReader is a
// GATED stub while ElevatedReader is the raw store. If elevated reads ever
// route through VisibleReader (the plausible "tidy up, they're both
// readers" refactor), these fail — that is what they exist to catch.

// gatedReader is a deliberately hostile VisibleReader: it hides SECRET-1
// entirely and strips the "classified" property off everything else. Its
// job is to be OBSERVABLY different from the raw store so a test can tell
// which reader a binding used.
type gatedReader struct{ raw store.Store }

// GetEntity reports a hidden entity as store.ErrNotFound, matching
// visibility.ScriptReader: a denial is indistinguishable from a genuine
// miss, so the gated path leaks no existence oracle. Returning (nil, nil)
// would violate the EntityReader contract and nil-deref the binding.
func (g gatedReader) GetEntity(ctx context.Context, id string) (*entity.Entity, error) {
	e, err := g.raw.GetEntity(ctx, id)
	if err != nil {
		return nil, err
	}
	red := g.redact(e)
	if red == nil {
		return nil, store.ErrNotFound
	}
	return red, nil
}

// redact returns a copy without the "classified" property, or nil for a
// hidden entity. Copying (not mutating) matters: the store hands out its
// own copies, but a future store that didn't would see its data corrupted
// by a reader, and this stub should not model that bug.
func (g gatedReader) redact(e *entity.Entity) *entity.Entity {
	if e.ID == "SECRET-1" {
		return nil
	}
	c := e.Clone()
	delete(c.Properties, "classified")
	return c
}

func (g gatedReader) ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	return func(yield func(*entity.Entity, error) bool) {
		for e, err := range g.raw.ListEntities(ctx, q) {
			if err != nil {
				if !yield(nil, err) {
					return
				}
				continue
			}
			if red := g.redact(e); red != nil && !yield(red, nil) {
				return
			}
		}
	}
}

// ListRelations drops any edge touching SECRET-1 — the peer-gating the real
// visibility reader does (RR-7GDT1Y).
func (g gatedReader) ListRelations(ctx context.Context, q store.RelationQuery) iter.Seq2[*entity.Relation, error] {
	return func(yield func(*entity.Relation, error) bool) {
		for rel, err := range g.raw.ListRelations(ctx, q) {
			if err != nil {
				if !yield(nil, err) {
					return
				}
				continue
			}
			if rel.From == "SECRET-1" || rel.To == "SECRET-1" {
				continue
			}
			if !yield(rel, nil) {
				return
			}
		}
	}
}

// recordingElevationRecorder captures the post-closure audit
// notifications, one entry per call.
type recordingElevationRecorder struct{ calls [][]string }

func (r *recordingElevationRecorder) RecordElevatedRead(_ context.Context, bindings []string) {
	// Copy: the binding owns the slice and is free to reuse it.
	r.calls = append(r.calls, append([]string(nil), bindings...))
}

// elevatedReadFixture builds a writer runtime whose gated view hides
// SECRET-1 and the "classified" property, while the elevated handle (when
// granted) reads the raw store.
//
// grantReads=false models a wiring site that gave write elevation but no
// read elevation — the nil-ElevatedReader deny path.
func elevatedReadFixture(t *testing.T, grantReads bool) *Runtime {
	t.Helper()
	r, _ := elevatedReadFixtureWithRecorder(t, grantReads)
	return r
}

// elevatedReadFixtureWithRecorder is elevatedReadFixture plus the audit
// recorder, returned so a test can inspect what was recorded.
func elevatedReadFixtureWithRecorder(
	t *testing.T, grantReads bool,
) (*Runtime, *recordingElevationRecorder) {
	t.Helper()
	ws := newMockWorkspace(t)
	ws.seedEntity(&entity.Entity{
		ID:   "SECRET-1",
		Type: "ticket",
		Properties: map[string]any{
			"title":      "Hidden Ticket",
			"status":     "open",
			"classified": "top-secret",
		},
	})
	ws.seedRelation(&entity.Relation{From: "SECRET-1", Type: "implements", To: "FEAT-001"})

	rec := &recordingElevationRecorder{}
	deps := ws.services("/tmp")
	deps.VisibleReader = gatedReader{raw: ws.store}
	deps.ElevatedManager = &recordingMutator{}
	deps.ElevationRecorder = rec
	if grantReads {
		deps.ElevatedReader = ws.store
	}
	var buf bytes.Buffer
	r := NewWriter(deps, &buf)
	t.Cleanup(r.Close)
	return r, rec
}

// TestElevatedRead_SeesWhatTheGatedReaderHides is the core contract: the
// same two reads, one inside the closure and one outside, disagree — and
// they disagree in the direction that proves elevation happened.
func TestElevatedRead_SeesWhatTheGatedReaderHides(t *testing.T) {
	t.Parallel()
	r := elevatedReadFixture(t, true)

	script := `
		-- Outside: gated. SECRET-1 is invisible and "classified" is stripped.
		if rela.get_entity("SECRET-1") ~= nil then
			error("gated read saw SECRET-1 -- the fixture is not actually gating")
		end
		local vis = rela.get_entity("TKT-001")
		if vis.properties.classified ~= nil then
			error("gated read kept the classified property")
		end

		-- Inside: raw.
		rela.bypass_acl(function(admin)
			local hidden = admin.get_entity("SECRET-1")
			if hidden == nil then
				error("elevated get_entity could not see the hidden entity")
			end
			if hidden.properties.classified ~= "top-secret" then
				error("elevated get_entity redacted the classified property: got "
					.. tostring(hidden.properties.classified))
			end
		end)
	`
	if err := r.RunString(script); err != nil {
		t.Fatalf("elevated read script: %v", err)
	}
}

// TestElevatedRead_ListEntitiesIsUngated pins the list surface: the gated
// binding drops the hidden row, the elevated one keeps it.
func TestElevatedRead_ListEntitiesIsUngated(t *testing.T) {
	t.Parallel()
	r := elevatedReadFixture(t, true)

	// Counting rather than comparing IDs: what must hold is that elevation
	// returns strictly MORE, and by exactly the hidden row.
	script := `
		local gated = #rela.list_entities("ticket")
		local elevated
		rela.bypass_acl(function(admin)
			elevated = #admin.list_entities("ticket")
		end)
		if elevated ~= gated + 1 then
			error("elevated list_entities returned " .. elevated ..
				" but gated returned " .. gated .. " -- expected exactly one more (SECRET-1)")
		end
	`
	if err := r.RunString(script); err != nil {
		t.Fatalf("elevated list script: %v", err)
	}
}

// TestElevatedRead_GetRelationsIsNotPeerGated pins that an edge to a hidden
// endpoint survives elevation. This is the surface most likely to be
// quietly re-gated, because the gated binding's peer-drop looks like a
// safety property worth keeping everywhere.
func TestElevatedRead_GetRelationsIsNotPeerGated(t *testing.T) {
	t.Parallel()
	r := elevatedReadFixture(t, true)

	script := `
		local function has_secret_edge(rels)
			for _, rel in ipairs(rels) do
				if rel.from == "SECRET-1" then return true end
			end
			return false
		end

		if has_secret_edge(rela.get_relations({to = "FEAT-001"})) then
			error("gated get_relations returned the SECRET-1 edge -- fixture is not gating")
		end
		rela.bypass_acl(function(admin)
			if not has_secret_edge(admin.get_relations({to = "FEAT-001"})) then
				error("elevated get_relations dropped the edge from the hidden entity")
			end
		end)
	`
	if err := r.RunString(script); err != nil {
		t.Fatalf("elevated relations script: %v", err)
	}
}

// TestElevatedRead_HandleInvalidatedAfterClosure pins that the read methods
// obey the SAME liveness guard as the write methods. A read that survives
// the closure would be a durable ungated capability — precisely what the
// object-capability design exists to prevent.
func TestElevatedRead_HandleInvalidatedAfterClosure(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		call string
	}{
		{"get_entity", `stashed.get_entity("SECRET-1")`},
		{"list_entities", `stashed.list_entities("ticket")`},
		{"get_relations", `stashed.get_relations({})`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := elevatedReadFixture(t, true)
			script := `
				local stashed
				rela.bypass_acl(function(admin) stashed = admin end)
			` + tc.call

			err := r.RunString(script)
			if err == nil {
				t.Fatal("an escaped admin handle performed an elevated READ after the " +
					"closure returned -- elevation leaked past its dynamic extent")
			}
			if !strings.Contains(err.Error(), "invalidated") {
				t.Errorf("error = %v, want it to mention the handle is invalidated", err)
			}
		})
	}
}

// TestElevatedRead_DeniesWhenNoReaderWired pins the nil-ElevatedReader
// contract: RAISE, never silently fall back to the gated VisibleReader.
//
// A fallback would be the worst outcome available — the closure would look
// elevated, return a partial graph, and the script would treat it as
// complete. Failing loudly is the only safe behavior for a capability that
// is supposed to be total.
func TestElevatedRead_DeniesWhenNoReaderWired(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		call string
	}{
		{"get_entity", `admin.get_entity("TKT-001")`},
		{"list_entities", `admin.list_entities("ticket")`},
		{"get_relations", `admin.get_relations({})`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := elevatedReadFixture(t, false) // write elevation only
			err := r.RunString(`rela.bypass_acl(function(admin) ` + tc.call + ` end)`)
			if err == nil {
				t.Fatal("an elevated read succeeded with NO elevated reader wired -- " +
					"it must have fallen back to the gated reader, which makes a " +
					"non-elevated result look elevated")
			}
			if !strings.Contains(err.Error(), "no elevated reader is configured") {
				t.Errorf("error = %v, want the 'no elevated reader is configured' raise", err)
			}
		})
	}
}

// TestElevatedRead_WriteElevationStillWorksWithoutReadElevation pins that
// the two capabilities are independent: withholding reads must not break
// the pre-existing TKT-D8T148 write surface.
func TestElevatedRead_WriteElevationStillWorksWithoutReadElevation(t *testing.T) {
	t.Parallel()
	r := elevatedReadFixture(t, false)
	err := r.RunString(`rela.bypass_acl(function(admin)
		admin.create_relation("alice", "created-by", "TKT-001")
	end)`)
	if err != nil {
		t.Fatalf("elevated WRITE broke when read elevation was withheld: %v", err)
	}
}

// TestElevatedRead_RejectsEmptyArguments pins argument validation, so a
// typo'd id reaches a clear Lua error rather than an empty-string store
// query whose result the script would misread as "not found".
func TestElevatedRead_RejectsEmptyArguments(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		call string
	}{
		{"get_entity", `admin.get_entity("")`},
		{"list_entities", `admin.list_entities("")`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := elevatedReadFixture(t, true)
			if err := r.RunString(`rela.bypass_acl(function(admin) ` + tc.call + ` end)`); err == nil {
				t.Error("an empty argument was accepted; want a raise")
			}
		})
	}
}

// TestElevatedRead_AbsentWithoutBypassBinding pins the outer gate: no
// allow_acl_bypass action means no rela.bypass_acl, so an ElevatedReader
// sitting in the deps grants a script nothing. Elevation needs BOTH keys.
func TestElevatedRead_AbsentWithoutBypassBinding(t *testing.T) {
	t.Parallel()
	ws := newMockWorkspace(t)
	deps := ws.services("/tmp")
	deps.VisibleReader = gatedReader{raw: ws.store}
	deps.ElevatedReader = ws.store // read capability present...
	deps.ElevatedManager = nil     // ...but no elevated Mutator: no binding.
	var buf bytes.Buffer
	r := NewWriter(deps, &buf)
	defer r.Close()

	if err := r.RunString(`if rela.bypass_acl ~= nil then error("bypass_acl present") end`); err != nil {
		t.Errorf("an ElevatedReader alone registered rela.bypass_acl (%v) -- read "+
			"elevation must not be a second, independent key to the closure", err)
	}
}

// TestElevatedRead_AuditsOncePerClosure pins the audit contract
// (TKT-ACSBSA): one record per closure that read, naming the distinct
// bindings used — NOT one per read.
//
// The per-read alternative is what makes this worth a test: a single
// admin.list_entities can traverse the whole graph, so per-row recording
// would put an unbounded synchronous write on a read path.
func TestElevatedRead_AuditsOncePerClosure(t *testing.T) {
	t.Parallel()
	r, rec := elevatedReadFixtureWithRecorder(t, true)

	// Two distinct bindings, and one of them called repeatedly.
	script := `
		rela.bypass_acl(function(admin)
			admin.get_entity("SECRET-1")
			admin.get_entity("TKT-001")
			admin.get_entity("TKT-002")
			admin.list_entities("ticket")
		end)
	`
	if err := r.RunString(script); err != nil {
		t.Fatalf("script: %v", err)
	}

	if len(rec.calls) != 1 {
		t.Fatalf("got %d audit records, want exactly 1 per closure "+
			"(4 reads happened; per-read recording would be unbounded)", len(rec.calls))
	}
	want := []string{"get_entity", "list_entities"}
	if !slices.Equal(rec.calls[0], want) {
		t.Errorf("recorded bindings = %v, want %v (distinct, first-use order; "+
			"get_entity was called 3x and must appear once)", rec.calls[0], want)
	}
}

// TestElevatedRead_AuditsEvenWhenClosureRaises is the anti-tampering
// property: a closure that reads raw data and THEN fails must still leave
// the trace.
//
// Recording only on the success path would let a script read everything and
// erase the evidence by raising — the shape an attacker would choose, and
// an easy mistake to make since the natural place for the call is after the
// PCall succeeds.
func TestElevatedRead_AuditsEvenWhenClosureRaises(t *testing.T) {
	t.Parallel()
	r, rec := elevatedReadFixtureWithRecorder(t, true)

	err := r.RunString(`
		rela.bypass_acl(function(admin)
			admin.get_entity("SECRET-1")
			error("boom")
		end)
	`)
	if err == nil {
		t.Fatal("expected the raise to propagate out of bypass_acl")
	}
	if len(rec.calls) != 1 {
		t.Fatalf("got %d audit records, want 1 -- a closure that read raw data and "+
			"then raised left NO trace, so failing is a way to erase the evidence",
			len(rec.calls))
	}
}

// TestElevatedRead_NoAuditWithoutReads pins the quiet case: a bypass_acl
// closure that only WRITES must not emit a read record. Those writes are
// already covered by entitymanager's OpACLBypass rows, and an empty
// read-record would add noise that makes the real ones harder to spot.
func TestElevatedRead_NoAuditWithoutReads(t *testing.T) {
	t.Parallel()
	r, rec := elevatedReadFixtureWithRecorder(t, true)

	err := r.RunString(`rela.bypass_acl(function(admin)
		admin.create_relation("alice", "created-by", "TKT-001")
	end)`)
	if err != nil {
		t.Fatalf("script: %v", err)
	}
	if len(rec.calls) != 0 {
		t.Errorf("a write-only closure emitted %d read audit records, want 0: %v",
			len(rec.calls), rec.calls)
	}
}

// TestElevatedRead_DeniedReadIsNotAudited pins that a REFUSED read (no
// elevated reader wired) emits no record. The record means "raw data was
// disclosed"; emitting one when nothing was read would make the audit log
// overstate what happened, which erodes trust in every other row.
func TestElevatedRead_DeniedReadIsNotAudited(t *testing.T) {
	t.Parallel()
	r, rec := elevatedReadFixtureWithRecorder(t, false)

	if err := r.RunString(
		`rela.bypass_acl(function(admin) admin.get_entity("TKT-001") end)`,
	); err == nil {
		t.Fatal("expected the denied read to raise")
	}
	if len(rec.calls) != 0 {
		t.Errorf("a DENIED elevated read was audited as though data was disclosed: %v",
			rec.calls)
	}
}

// TestElevatedRead_NilRecorderIsNotFatal pins that a missing audit sink
// degrades quietly. Unlike a missing ElevatedReader (which denies), a
// deployment may legitimately run without an audit sink, and refusing to
// run automations over that would be a worse failure than an unrecorded
// read.
func TestElevatedRead_NilRecorderIsNotFatal(t *testing.T) {
	t.Parallel()
	ws := newMockWorkspace(t)
	deps := ws.services("/tmp")
	deps.VisibleReader = gatedReader{raw: ws.store}
	deps.ElevatedManager = &recordingMutator{}
	deps.ElevatedReader = ws.store
	deps.ElevationRecorder = nil // no sink wired
	var buf bytes.Buffer
	r := NewWriter(deps, &buf)
	defer r.Close()

	if err := r.RunString(
		`rela.bypass_acl(function(admin) admin.get_entity("TKT-001") end)`,
	); err != nil {
		t.Errorf("an elevated read failed because no audit sink was wired: %v -- "+
			"recording is best-effort, not a precondition", err)
	}
}
