package lua

import (
	"bytes"
	"context"
	"errors"
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

// failingElevatedReader yields one good row and then an error, to exercise
// the mid-iteration failure path. The panic after the failing yield is the
// point: if s.RaiseError did NOT unwind the range loop, the iterator would be
// resumed and the panic would fire.
type failingElevatedReader struct{ raw store.Store }

func (f failingElevatedReader) GetEntity(ctx context.Context, id string) (*entity.Entity, error) {
	return f.raw.GetEntity(ctx, id)
}

func (f failingElevatedReader) ListEntities(
	context.Context, store.EntityQuery,
) iter.Seq2[*entity.Entity, error] {
	return func(yield func(*entity.Entity, error) bool) {
		if !yield(&entity.Entity{ID: "A", Type: "ticket"}, nil) {
			return
		}
		yield(nil, errors.New("synthetic store failure"))
		panic("iterator resumed after RaiseError -- the longjmp did not unwind " +
			"the range loop, so the binding is leaking an in-flight iterator")
	}
}

func (f failingElevatedReader) ListRelations(
	context.Context, store.RelationQuery,
) iter.Seq2[*entity.Relation, error] {
	return func(func(*entity.Relation, error) bool) {}
}

// brokenElevatedReader fails every GetEntity with an INFRASTRUCTURE error
// (not a miss), to pin that such errors surface rather than masquerading as
// "does not exist".
type brokenElevatedReader struct{}

func (brokenElevatedReader) GetEntity(context.Context, string) (*entity.Entity, error) {
	return nil, errors.New("connection refused: database is down")
}

func (brokenElevatedReader) ListEntities(
	context.Context, store.EntityQuery,
) iter.Seq2[*entity.Entity, error] {
	return func(func(*entity.Entity, error) bool) {}
}

func (brokenElevatedReader) ListRelations(
	context.Context, store.RelationQuery,
) iter.Seq2[*entity.Relation, error] {
	return func(func(*entity.Relation, error) bool) {}
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

// TestElevatedRead_StoreErrorMidIterationUnwindsCleanly pins two properties
// of the failure path that are easy to get wrong together.
//
// The elevated list bindings call s.RaiseError from INSIDE a range-over-func
// loop -- unlike the gated bindings, which `break` on an iterator error. That
// is a longjmp out of a running iterator, so this pins that (a) the range loop
// really unwinds rather than resuming the iterator (the fixture panics if it
// is resumed), (b) the store error reaches the script instead of being
// silently swallowed into a short result -- a truncated list that looked
// complete would be worse than an error -- and (c) the audit defer still
// fires, so a read that partially succeeded before failing is still recorded.
func TestElevatedRead_StoreErrorMidIterationUnwindsCleanly(t *testing.T) {
	t.Parallel()
	ws := newMockWorkspace(t)
	rec := &recordingElevationRecorder{}
	deps := ws.services("/tmp")
	deps.VisibleReader = gatedReader{raw: ws.store}
	deps.ElevatedManager = &recordingMutator{}
	deps.ElevatedReader = failingElevatedReader{raw: ws.store}
	deps.ElevationRecorder = rec
	var buf bytes.Buffer
	r := NewWriter(deps, &buf)
	defer r.Close()

	err := r.RunString(`rela.bypass_acl(function(admin) admin.list_entities("ticket") end)`)
	if err == nil {
		t.Fatal("a store failure mid-iteration was swallowed -- the script got a " +
			"TRUNCATED list it would read as complete")
	}
	if !strings.Contains(err.Error(), "synthetic store failure") {
		t.Errorf("error = %v, want it to carry the underlying store failure", err)
	}
	if len(rec.calls) != 1 {
		t.Errorf("got %d audit records, want 1 -- the read partially succeeded "+
			"before failing, so it must still be recorded", len(rec.calls))
	}
}

// TestElevatedRead_StoreErrorIsNotMaskedAsMissing pins that only a genuine
// MISS yields nil (RR: significant).
//
// Masking an infrastructure error as nil breaks the documented contract
// ("admin.get_entity returns nil only when the entity genuinely does not
// exist") and silently breaks the motivating use case: the guide's own
// example is a cross-entity uniqueness check, and a nil returned during a
// transient outage reads as "no duplicate" — so the invariant the elevated
// read exists to ENFORCE gets violated, quietly, on exactly the occasions
// the system is already unhealthy.
func TestElevatedRead_StoreErrorIsNotMaskedAsMissing(t *testing.T) {
	t.Parallel()
	ws := newMockWorkspace(t)
	deps := ws.services("/tmp")
	deps.VisibleReader = gatedReader{raw: ws.store}
	deps.ElevatedManager = &recordingMutator{}
	deps.ElevatedReader = brokenElevatedReader{}
	var buf bytes.Buffer
	r := NewWriter(deps, &buf)
	defer r.Close()

	err := r.RunString(`rela.bypass_acl(function(admin) admin.get_entity("TKT-001") end)`)
	if err == nil {
		t.Fatal("a store OUTAGE was reported to the script as nil (not-found) -- " +
			"a uniqueness check would read that as 'no duplicate' and admit one")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("error = %v, want it to carry the underlying store failure", err)
	}
}

// TestElevatedRead_MissStillReturnsNil is the other half: a genuine miss must
// stay nil, or every "does this exist?" script breaks.
func TestElevatedRead_MissStillReturnsNil(t *testing.T) {
	t.Parallel()
	r := elevatedReadFixture(t, true)
	if err := r.RunString(`rela.bypass_acl(function(admin)
		if admin.get_entity("NOPE-404") ~= nil then
			error("a missing entity did not come back as nil")
		end
	end)`); err != nil {
		t.Fatalf("a genuine miss raised instead of returning nil: %v", err)
	}
}

// TestElevatedRead_DeniedByArgValidationIsNotAudited pins that the audit row
// means what it says (RR: significant).
//
// The empty-argument checks raise BEFORE the reader is ever called, so no
// data was disclosed. Recording one anyway would make the audit log overstate
// what happened — the same principle
// TestElevatedRead_DeniedReadIsNotAudited pins for the nil-reader denial,
// which is why the mark happens at the store call rather than in readGuard.
func TestElevatedRead_DeniedByArgValidationIsNotAudited(t *testing.T) {
	t.Parallel()
	r, rec := elevatedReadFixtureWithRecorder(t, true)

	// pcall so the raises do not abort the closure; the point is what got
	// recorded, not that they raised (covered separately).
	if err := r.RunString(`rela.bypass_acl(function(admin)
		pcall(function() admin.get_entity("") end)
		pcall(function() admin.list_entities("") end)
	end)`); err != nil {
		t.Fatalf("script: %v", err)
	}

	if len(rec.calls) != 0 {
		t.Errorf("got %d audit records (%v), want 0 -- these calls were rejected "+
			"by argument validation and never reached the store, so recording "+
			"them claims a disclosure that did not happen", len(rec.calls), rec.calls)
	}
}

// TestElevatedRead_GetRelationsRejectsNonStringFilter pins that a mistyped
// filter fails loudly instead of becoming a whole-graph scan (RR: minor).
//
// `{from = 12345}` — an id that came back as a number, or a typo'd key —
// previously dropped the constraint silently and returned EVERY edge, which
// the script would read as a filtered result. On the gated path an
// over-broad query is still peer-gated by the reader; here nothing gates it.
func TestElevatedRead_GetRelationsRejectsNonStringFilter(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct{ name, opts string }{
		{"numeric from", `{from = 12345}`},
		{"boolean type", `{type = true}`},
		{"table to", `{to = {}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := elevatedReadFixture(t, true)
			err := r.RunString(
				`rela.bypass_acl(function(admin) admin.get_relations(` + tc.opts + `) end)`)
			if err == nil {
				t.Fatal("a non-string filter was silently DROPPED -- the call returned " +
					"an unfiltered whole-graph edge dump that the script reads as filtered")
			}
			if !strings.Contains(err.Error(), "must be a string") {
				t.Errorf("error = %v, want it to name the bad option type", err)
			}
		})
	}
}

// TestElevatedRead_GetRelationsAcceptsAbsentFilters pins that omitting the
// options (or individual keys) still means "no constraint" — the rejection
// above must not have turned absent into invalid.
func TestElevatedRead_GetRelationsAcceptsAbsentFilters(t *testing.T) {
	t.Parallel()
	r := elevatedReadFixture(t, true)
	if err := r.RunString(`rela.bypass_acl(function(admin)
		admin.get_relations()
		admin.get_relations({})
		admin.get_relations({from = "TKT-001"})
	end)`); err != nil {
		t.Fatalf("absent or partial filters were rejected: %v", err)
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
