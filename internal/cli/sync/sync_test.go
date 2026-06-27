package sync

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/canonical"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// preconditionOK mirrors the server's push precondition: with an If-Match the
// current hash must match; with none, the record must not yet exist. Kept local
// to the fake server so the test asserts the contract independently of the
// server's copy.
func preconditionOK(ifMatch, currentHash string, exists bool) bool {
	if ifMatch == "" {
		return !exists
	}
	return exists && ifMatch == currentHash
}

// fakeServer is an in-memory stand-in for the rela-server /api/sync/ API. It
// implements the same conditional (If-Match) semantics as the real handlers so
// the client/engine are exercised against the true wire contract: 200 + ETag on
// apply, 412 on a precondition mismatch, 404 on absent, manifest with a seq
// cursor. It is intentionally independent of the server code so a divergence in
// the contract shows up as a test failure here.
type fakeServer struct {
	mu        sync.Mutex
	entities  map[string]*entity.Entity
	relations map[string]*entity.Relation
	seq       int64
	changes   []serverChange // append-only change log for the manifest
	authToken string         // when set, requests must present it as a bearer
}

type serverChange struct {
	seq     int64
	kind    string // "e"/"r"
	key     string
	typ     string
	deleted bool
}

func newFakeServer() *fakeServer {
	return &fakeServer{entities: map[string]*entity.Entity{}, relations: map[string]*entity.Relation{}}
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
	case strings.HasPrefix(r.URL.Path, "/api/sync/entities/"):
		s.record(w, r, "e", strings.TrimPrefix(r.URL.Path, "/api/sync/entities/"))
	case strings.HasPrefix(r.URL.Path, "/api/sync/relations/"):
		s.record(w, r, "r", strings.TrimPrefix(r.URL.Path, "/api/sync/relations/"))
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

func (s *fakeServer) record(w http.ResponseWriter, r *http.Request, kind, key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch r.Method {
	case http.MethodGet:
		s.get(w, kind, key)
	case http.MethodPut:
		s.put(w, r, kind, key)
	case http.MethodDelete:
		s.del(w, r, kind, key)
	default:
		writeJSONErr(w, http.StatusMethodNotAllowed, "method_not_allowed")
	}
}

func (s *fakeServer) get(w http.ResponseWriter, kind, key string) {
	if kind == "e" {
		e, ok := s.entities[key]
		if !ok {
			writeJSONErr(w, http.StatusNotFound, "not_found")
			return
		}
		w.Header().Set("ETag", canonical.HashEntity(*e))
		_ = json.NewEncoder(w).Encode(EntityBody{ID: e.ID, Type: e.Type, Properties: e.Properties, Content: e.Content})
		return
	}
	rel, ok := s.relations[key]
	if !ok {
		writeJSONErr(w, http.StatusNotFound, "not_found")
		return
	}
	w.Header().Set("ETag", canonical.HashRelation(*rel))
	_ = json.NewEncoder(w).Encode(RelationBody{From: rel.From, Type: rel.Type, To: rel.To, Properties: rel.Properties, Content: rel.Content})
}

func (s *fakeServer) put(w http.ResponseWriter, r *http.Request, kind, key string) {
	ifMatch := r.Header.Get("If-Match")
	cur, exists := s.currentHash(kind, key)
	if !preconditionOK(ifMatch, cur, exists) {
		if exists {
			w.Header().Set("ETag", cur)
		}
		writeJSONErr(w, http.StatusPreconditionFailed, "conflict")
		return
	}
	if kind == "e" {
		var b EntityBody
		_ = json.NewDecoder(r.Body).Decode(&b)
		if b.Type == "invalid" { // a hook to force a 422 in tests
			writeJSONErr(w, http.StatusUnprocessableEntity, "validation_failed")
			return
		}
		e := &entity.Entity{ID: key, Type: b.Type, Properties: b.Properties, Content: b.Content}
		s.entities[key] = e
		s.recordChange("e", key, b.Type, false)
		h := canonical.HashEntity(*e)
		w.Header().Set("ETag", h)
		_ = json.NewEncoder(w).Encode(map[string]string{"hash": h})
		return
	}
	from, relType, to := split3(key)
	var b RelationBody
	_ = json.NewDecoder(r.Body).Decode(&b)
	rel := &entity.Relation{From: from, Type: relType, To: to, Properties: b.Properties, Content: b.Content}
	s.relations[key] = rel
	s.recordChange("r", key, "", false)
	h := canonical.HashRelation(*rel)
	w.Header().Set("ETag", h)
	_ = json.NewEncoder(w).Encode(map[string]string{"hash": h})
}

func (s *fakeServer) del(w http.ResponseWriter, r *http.Request, kind, key string) {
	ifMatch := r.Header.Get("If-Match")
	cur, exists := s.currentHash(kind, key)
	if !exists {
		writeJSONErr(w, http.StatusNotFound, "not_found")
		return
	}
	if ifMatch == "" || ifMatch != cur {
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

func split3(key string) (a, b, c string) {
	p := strings.Split(key, "/")
	return p[0], p[1], p[2]
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
