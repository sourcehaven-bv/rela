package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/canonical"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// fakeServer is an in-memory stand-in for the rela-server API the sync client
// speaks to (TKT-8P1TM7): the /api/v1 read+write routes plus /api/sync/manifest.
// It serves _schema (type↔plural), entity GET/POST/PATCH/DELETE and the
// single-relation GET/PATCH/DELETE with the same conditional (If-Match) and
// redaction semantics the real handlers use — 200+ETag on apply, 201+minted-id
// on create, 412 on precondition mismatch, 404 on absent, `_redacted` names on
// reads. It is intentionally independent of the server code so a divergence in
// the contract shows up as a test failure here.
type fakeServer struct {
	mu        sync.Mutex
	entities  map[string]*entity.Entity
	relations map[string]*entity.Relation
	seq       int64
	changes   []serverChange // append-only change log for the manifest
	authToken string         // when set, requests must present it as a bearer
	// types maps entity type → plural (and the reverse via pluralToType). The
	// schema handshake serves it; routing resolves plural→type. Seeded by
	// registerType so tests declare only the types they use.
	types        map[string]string // type → plural
	pluralToType map[string]string // plural → type
	// nextID mints ids for POST creates (the primary owns ids). Per-prefix
	// counter keyed by type so minted ids read clearly in assertions.
	createSeq int
	// redact, when set for a type+property, withholds that property's VALUE from
	// reads and lists it in `_redacted` (models field-level `visible:` ACL).
	redact map[string]bool // "type/prop" → hidden
	// conflictOnceKey, when set, forces the FIRST write of that key to respond
	// 409 Conflict (a create that raced a concurrent first-create on the
	// multi-writer backend), then behaves normally.
	conflictOnceKey string
}

type serverChange struct {
	seq     int64
	kind    string // "e"/"r"
	key     string
	typ     string
	deleted bool
}

func newFakeServer() *fakeServer {
	s := &fakeServer{
		entities:     map[string]*entity.Entity{},
		relations:    map[string]*entity.Relation{},
		types:        map[string]string{},
		pluralToType: map[string]string{},
		redact:       map[string]bool{},
	}
	// The tests use these entity types and one relation type.
	s.registerType("ticket", "tickets")
	s.registerType("decision", "decisions")
	s.registerType("blocks", "blocks") // relation type; plural unused but harmless
	return s
}

// registerType declares a type↔plural pair for the schema handshake + routing.
func (s *fakeServer) registerType(typ, plural string) {
	s.types[typ] = plural
	s.pluralToType[plural] = typ
}

// hideField marks type.prop as field-level redacted: reads withhold its value
// and list it in `_redacted`.
func (s *fakeServer) hideField(typ, prop string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.redact[typ+"/"+prop] = true
}

func (s *fakeServer) start(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(s.handle))
	t.Cleanup(srv.Close)
	return srv
}

func (s *fakeServer) handle(w http.ResponseWriter, r *http.Request) {
	if s.authToken != "" {
		if r.Header.Get("Authorization") != "Bearer "+s.authToken {
			writeJSONErr(w, http.StatusForbidden, "forbidden")
			return
		}
	}
	switch {
	case r.URL.Path == "/api/sync/manifest":
		s.manifest(w, r)
	case r.URL.Path == "/api/v1/_schema":
		s.serveSchema(w)
	case strings.HasPrefix(r.URL.Path, "/api/v1/"):
		s.serveV1(w, r, strings.TrimPrefix(r.URL.Path, "/api/v1/"))
	default:
		writeJSONErr(w, http.StatusNotFound, "not_found")
	}
}

// serveSchema serves the type↔plural map the client fetches once per run.
func (s *fakeServer) serveSchema(w http.ResponseWriter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	type et struct {
		Plural     string                    `json:"plural"`
		Properties map[string]map[string]any `json:"properties"`
	}
	out := struct {
		Entities  map[string]et `json:"entities"`
		Relations map[string]et `json:"relations"`
	}{Entities: map[string]et{}, Relations: map[string]et{}}
	for typ, plural := range s.types {
		out.Entities[typ] = et{Plural: plural, Properties: map[string]map[string]any{}}
	}
	_ = json.NewEncoder(w).Encode(out)
}

// serveV1 routes /api/v1/{plural}/{id}[/relations/{relType}/{to}].
func (s *fakeServer) serveV1(w http.ResponseWriter, r *http.Request, rest string) {
	parts := strings.Split(rest, "/")
	// {plural}                              → create (POST)
	// {plural}/{id}                         → entity get/patch/delete
	// {plural}/{id}/relations/{type}/{to}   → relation get/patch/delete
	s.mu.Lock()
	defer s.mu.Unlock()

	switch {
	case len(parts) == 1 && r.Method == http.MethodPost:
		s.createEntity(w, r, parts[0])
	case len(parts) == 2:
		s.entityByID(w, r, parts[1])
	case len(parts) == 5 && parts[2] == "relations":
		s.relationByTriple(w, r, parts[1], parts[3], parts[4])
	default:
		writeJSONErr(w, http.StatusNotFound, "not_found")
	}
}

func (s *fakeServer) manifest(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cursor, _ := strconv.ParseInt(r.URL.Query().Get("cursor"), 10, 64)
	resp := manifestResponse{Cursor: strconv.FormatInt(s.seq, 10)}
	for _, c := range s.changes {
		if c.seq <= cursor {
			continue
		}
		resp.Changes = append(resp.Changes, ManifestChange{Kind: c.kind, ID: c.key, Typ: c.typ, Deleted: c.deleted})
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// createEntity models POST /api/v1/{plural}: the primary MINTS the id and
// returns it (201 + Location + body{id}). A 409 hook models a create race.
func (s *fakeServer) createEntity(w http.ResponseWriter, r *http.Request, plural string) {
	typ, ok := s.pluralToType[plural]
	if !ok {
		writeJSONErr(w, http.StatusNotFound, "not_found")
		return
	}
	var b v1CreateEntity
	_ = json.NewDecoder(r.Body).Decode(&b)
	if b.Properties["type"] == "invalid" { // a hook to force a 422 in tests
		writeJSONErr(w, http.StatusUnprocessableEntity, "validation_failed")
		return
	}
	s.createSeq++
	// A minted-id prefix for readable test assertions, built from the plural (an
	// id prefix, not a derived label — see DEC-6C1NAA).
	prefix := s.types[typ]
	if prefix == "" {
		prefix = typ
	}
	id := fmt.Sprintf("%s-%d", prefix, s.createSeq)
	if s.conflictOnceKey == id || s.conflictOnceKey == "*create*" {
		s.conflictOnceKey = ""
		writeJSONErr(w, http.StatusConflict, "conflict")
		return
	}
	e := &entity.Entity{ID: id, Type: typ, Properties: b.Properties, Content: b.Content}
	s.entities[id] = e
	s.recordChange("e", id, typ, false)
	w.Header().Set("ETag", canonical.HashEntity(*e))
	w.Header().Set("Location", "/api/v1/"+plural+"/"+id)
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{"id": id})
}

// entityByID models GET/PATCH/DELETE /api/v1/{plural}/{id}.
func (s *fakeServer) entityByID(w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet:
		e, ok := s.entities[id]
		if !ok {
			writeJSONErr(w, http.StatusNotFound, "not_found")
			return
		}
		w.Header().Set("ETag", canonical.HashEntity(*e)) // ETag over RAW record
		visible, redacted := s.redactProps(e.Type, e.Properties)
		_ = json.NewEncoder(w).Encode(v1EntityResponse{
			ID: e.ID, Type: e.Type, Properties: visible, Content: e.Content, Redacted: redacted,
		})
	case http.MethodPatch:
		s.patchEntity(w, r, id)
	case http.MethodDelete:
		s.deleteRecord(w, r, "e", id)
	default:
		writeJSONErr(w, http.StatusMethodNotAllowed, "method_not_allowed")
	}
}

// patchEntity models PATCH /api/v1/{plural}/{id}: merge named properties onto
// the raw stored record (unnamed preserved), apply properties_unset, under
// If-Match on the RAW-record ETag.
func (s *fakeServer) patchEntity(w http.ResponseWriter, r *http.Request, id string) {
	e, exists := s.entities[id]
	if !exists {
		writeJSONErr(w, http.StatusNotFound, "not_found")
		return
	}
	cur := canonical.HashEntity(*e)
	if im := r.Header.Get("If-Match"); im != "" && im != cur {
		w.Header().Set("ETag", cur)
		writeJSONErr(w, http.StatusPreconditionFailed, "conflict")
		return
	}
	var b v1PatchEntity
	_ = json.NewDecoder(r.Body).Decode(&b)
	if b.Properties == nil {
		b.Properties = map[string]any{}
	}
	merged := map[string]any{}
	maps.Copy(merged, e.Properties)
	maps.Copy(merged, b.Properties)
	for _, k := range b.PropertiesUnset {
		delete(merged, k)
	}
	e.Properties = merged
	if b.Content != nil {
		e.Content = *b.Content
	}
	s.recordChange("e", id, e.Type, false)
	w.Header().Set("ETag", canonical.HashEntity(*e))
	_ = json.NewEncoder(w).Encode(map[string]string{"id": id})
}

// relationByTriple models GET/PATCH/DELETE
// /api/v1/{plural}/{from}/relations/{type}/{to}.
func (s *fakeServer) relationByTriple(w http.ResponseWriter, r *http.Request, from, relType, to string) {
	key := from + "/" + relType + "/" + to
	switch r.Method {
	case http.MethodGet:
		rel, ok := s.relations[key]
		if !ok {
			writeJSONErr(w, http.StatusNotFound, "not_found")
			return
		}
		w.Header().Set("ETag", canonical.HashRelation(*rel))
		visible, redacted := s.redactProps(relType, rel.Properties)
		_ = json.NewEncoder(w).Encode(relationReadWire{
			From: from, Type: relType, To: to, Meta: visible, Content: rel.Content, Redacted: redacted,
		})
	case http.MethodPatch:
		s.putRelation(w, r, from, relType, to, key)
	case http.MethodDelete:
		s.deleteRecord(w, r, "r", key)
	default:
		writeJSONErr(w, http.StatusMethodNotAllowed, "method_not_allowed")
	}
}

// putRelation models the relation upsert (create-or-update, idempotent on the
// triple) under If-Match on the raw relation ETag.
func (s *fakeServer) putRelation(w http.ResponseWriter, r *http.Request, from, relType, to, key string) {
	cur, exists := s.currentHash("r", key)
	if im := r.Header.Get("If-Match"); im != "" && (!exists || im != cur) {
		if exists {
			w.Header().Set("ETag", cur)
		}
		writeJSONErr(w, http.StatusPreconditionFailed, "conflict")
		return
	}
	var b v1RelationWrite
	_ = json.NewDecoder(r.Body).Decode(&b)
	rel := &entity.Relation{From: from, Type: relType, To: to, Properties: b.Meta}
	s.relations[key] = rel
	s.recordChange("r", key, relType, false)
	w.Header().Set("ETag", canonical.HashRelation(*rel))
	_ = json.NewEncoder(w).Encode(map[string]string{"id": to})
}

// deleteRecord models DELETE under If-Match on the raw ETag.
func (s *fakeServer) deleteRecord(w http.ResponseWriter, r *http.Request, kind, key string) {
	cur, exists := s.currentHash(kind, key)
	if !exists {
		writeJSONErr(w, http.StatusNotFound, "not_found")
		return
	}
	if im := r.Header.Get("If-Match"); im == "" || im != cur {
		w.Header().Set("ETag", cur)
		writeJSONErr(w, http.StatusPreconditionFailed, "conflict")
		return
	}
	if kind == "e" {
		delete(s.entities, key)
	} else {
		delete(s.relations, key)
	}
	s.recordChange(kind, key, "", true)
	_ = json.NewEncoder(w).Encode(map[string]string{"deleted": key})
}

// redactProps splits a property map into the visible values and the sorted
// names of the redacted ones, per the fake's redact set (models field ACL).
// Always returns a non-nil _redacted slice (the closed-world signal).
func (s *fakeServer) redactProps(typ string, props map[string]any) (visible map[string]any, redacted *[]string) {
	visible = map[string]any{}
	names := []string{}
	for k, v := range props {
		if s.redact[typ+"/"+k] {
			names = append(names, k)
			continue
		}
		visible[k] = v
	}
	sort.Strings(names)
	return visible, &names
}

// relationReadWire is the fake's single-relation read shape (matches the real
// relationReadResponse the client decodes as v1RelationResponse).
type relationReadWire struct {
	From     string         `json:"from"`
	Type     string         `json:"type"`
	To       string         `json:"to"`
	Meta     map[string]any `json:"meta,omitempty"`
	Content  string         `json:"content,omitempty"`
	Redacted *[]string      `json:"_redacted,omitempty"`
}

func (s *fakeServer) currentHash(kind, key string) (string, bool) {
	if kind == "e" {
		e, ok := s.entities[key]
		if !ok {
			return "", false
		}
		return canonical.HashEntity(*e), true
	}
	rel, ok := s.relations[key]
	if !ok {
		return "", false
	}
	return canonical.HashRelation(*rel), true
}

func (s *fakeServer) recordChange(kind, key, typ string, deleted bool) {
	s.seq++
	s.changes = append(s.changes, serverChange{seq: s.seq, kind: kind, key: key, typ: typ, deleted: deleted})
}

// seedEntity / seedRelation add a record directly (server-side create), bumping
// the change log so a pull sees it.
func (s *fakeServer) seedEntity(id, typ string, props map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entities[id] = &entity.Entity{ID: id, Type: typ, Properties: props}
	s.recordChange("e", id, typ, false)
}

func writeJSONErr(w http.ResponseWriter, code int, reason string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "error", "reason": reason})
}

// --- local side: memstore + a fake applier that writes through to it ---

// memApplier satisfies LocalApplier by writing id-preserving upserts straight
// into a memstore — the sync-relevant behavior of entitymanager.ApplyEntity
// without the validation/automation machinery (those are tested in
// entitymanager's own suite).
type memApplier struct{ st *memstore.MemStore }

func (a memApplier) ApplyEntity(ctx context.Context, e *entity.Entity) (*entity.UpdateResult, error) {
	if _, err := a.st.GetEntity(ctx, e.ID); err == nil {
		return nil, a.st.UpdateEntity(ctx, e)
	}
	return nil, a.st.CreateEntity(ctx, e)
}

func (a memApplier) ApplyRelation(ctx context.Context, r *entity.Relation) (*entity.Relation, error) {
	data := store.RelationData{Properties: r.Properties, Content: r.Content}
	if _, err := a.st.GetRelation(ctx, r.From, r.Type, r.To); err == nil {
		return a.st.UpdateRelation(ctx, r.From, r.Type, r.To, data)
	}
	return a.st.CreateRelation(ctx, r.From, r.Type, r.To, &data)
}

func (a memApplier) DeleteEntity(ctx context.Context, id string, cascade bool) (*entity.DeleteResult, error) {
	if _, err := a.st.DeleteEntity(ctx, id, cascade); err != nil {
		return nil, err
	}
	return &entity.DeleteResult{}, nil
}

func (a memApplier) DeleteRelation(ctx context.Context, from, relType, to string) error {
	return a.st.DeleteRelation(ctx, from, relType, to)
}

func (a memApplier) RenameEntity(
	ctx context.Context, oldID, newID string, _ entity.RenameOptions,
) (*entity.RenameResult, error) {
	res, err := a.st.RenameEntity(ctx, oldID, newID)
	if err != nil {
		return nil, err
	}
	return &entity.RenameResult{OldID: oldID, NewID: newID, RelationsUpdated: res.RelationsUpdated}, nil
}

// harness bundles a local store, a fake server, and an engine over them.
type harness struct {
	st     *memstore.MemStore
	server *fakeServer
	engine *Engine
	idx    *State
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	st := memstore.New()
	fs := newFakeServer()
	srv := fs.start(t)
	client, err := NewClient(srv.URL, "", nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	idx := newState()
	eng, err := NewEngine(client, st, memApplier{st: st}, idx)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	return &harness{st: st, server: fs, engine: eng, idx: idx}
}

func (h *harness) createLocalEntity(t *testing.T, id string, props map[string]any) {
	t.Helper()
	if err := h.st.CreateEntity(context.Background(), &entity.Entity{ID: id, Type: "ticket", Properties: props}); err != nil {
		t.Fatalf("create local entity: %v", err)
	}
}

// onlyServerEntityID returns the id of the single entity on the fake primary,
// failing if there is not exactly one. Used after a push-create to discover the
// primary-minted id (the replica does not choose it).
func (h *harness) onlyServerEntityID(t *testing.T) string {
	t.Helper()
	h.server.mu.Lock()
	defer h.server.mu.Unlock()
	if len(h.server.entities) != 1 {
		t.Fatalf("want exactly 1 server entity, got %d", len(h.server.entities))
	}
	for id := range h.server.entities {
		return id
	}
	return ""
}
