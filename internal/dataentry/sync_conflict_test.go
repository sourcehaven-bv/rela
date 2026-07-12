package dataentry

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// createConflictStore reports every id as ABSENT via GetEntity — so the sync
// PUT precondition (no If-Match on a non-existent record) passes AND ApplyEntity
// resolves CREATE intent — but conflicts on CreateEntity, modeling a concurrent
// create landing on the postgres multi-writer backend between probe and write.
// It embeds a real memstore so the non-overridden methods (search backfill,
// relation reads) behave; only the two methods that drive the race are stubbed.
type createConflictStore struct {
	store.Store
}

func (s *createConflictStore) GetEntity(_ context.Context, _ string) (*entity.Entity, error) {
	return nil, store.ErrNotFound
}

func (s *createConflictStore) CreateEntity(_ context.Context, _ *entity.Entity) error {
	return store.ErrConflict
}

// vanishOnUpdateStore reports a fixed entity PRESENT via GetEntity — so the
// sync PUT precondition (If-Match matching the current hash) passes AND
// ApplyEntity resolves UPDATE intent — but returns store.ErrNotFound on
// UpdateEntity, modeling a concurrent DELETE landing between the probe and the
// durable write. This is the narrow probe-said-present-then-deleted race; the
// server must map it to 412 (symmetric with the relation vanished-on-update
// case), not 404.
type vanishOnUpdateStore struct {
	store.Store
	present *entity.Entity
}

func (s *vanishOnUpdateStore) GetEntity(_ context.Context, id string) (*entity.Entity, error) {
	if id == s.present.ID {
		return s.present, nil
	}
	return nil, store.ErrNotFound
}

func (s *vanishOnUpdateStore) UpdateEntity(_ context.Context, _ *entity.Entity) error {
	return store.ErrNotFound
}

// TestSyncPut_UpdateVanishedReturns412 pins M1 (BUG-ZWTDH9): an update-intent
// sync PUT (If-Match matches, so the record probed as present) whose durable
// UpdateEntity finds the row gone (a concurrent delete) returns 412 Conflict —
// symmetric with the relation vanished-on-update case — NOT the 404 that would
// abort the CLI push run. The narrow race the ordinary preconditionOK->412
// check does not catch.
func TestSyncPut_UpdateVanishedReturns412(t *testing.T) {
	meta := secretNoteMeta()
	cfg := &Config{App: AppConfig{Name: "sec"}}
	app := newAppFromParts(cfg, meta, &fixture{})

	fs := storage.NewMemFS()
	ctx := &project.Context{Root: "/project", CacheDir: "/project/.rela"}
	_ = fs.MkdirAll(ctx.CacheDir, 0o755)

	present := &entity.Entity{ID: "NOTE-1", Type: "note", Properties: map[string]any{"title": "base"}}
	st := &vanishOnUpdateStore{Store: memstore.New(), present: present}
	svc := appbuildtest.New(meta, appbuildtest.WithFS(fs, ctx), appbuildtest.WithStore(st), appbuildtest.WithACL(acl.NopACL{}))
	rebindApp(app, fs, ctx, svc)
	app.broker = newEventBroker()
	app.SetPrincipalResolver(func(*http.Request) principal.Principal {
		return principal.Principal{User: "peer", Tool: principal.ToolSync}
	})

	// If-Match matches the current hash → precondition passes and ApplyEntity
	// resolves UPDATE intent. The conflict only surfaces on UpdateEntity.
	cur, exists := app.currentEntityHash(context.Background(), "NOTE-1")
	if !exists {
		t.Fatal("seed missing: store should report NOTE-1 present")
	}
	body := syncEntityBody{ID: "NOTE-1", Type: "note", Properties: map[string]any{"title": "edited"}}
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	r := httptest.NewRequest(http.MethodPut, "/api/sync/entities/NOTE-1", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("If-Match", cur)
	w := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(w, r)

	if w.Code != http.StatusPreconditionFailed {
		t.Fatalf("update-vanished sync PUT returned %d, want 412: %s", w.Code, w.Body.String())
	}
}

// TestSyncPut_CreateConflictReturns409 pins that when a create-intent sync PUT
// (no If-Match, record probes as absent) reaches ApplyEntity and the durable
// CreateEntity conflicts (a peer created the same id concurrently), the handler
// returns 409 Conflict — NOT a 412 precondition (the client's precondition was
// satisfied at check time), NOT a 422 (the content is valid), and NOT a silent
// 200 overwrite (the removed upsert fallback). BUG-ZWTDH9.
func TestSyncPut_CreateConflictReturns409(t *testing.T) {
	meta := secretNoteMeta()
	cfg := &Config{App: AppConfig{Name: "sec"}}
	app := newAppFromParts(cfg, meta, &fixture{})

	fs := storage.NewMemFS()
	ctx := &project.Context{Root: "/project", CacheDir: "/project/.rela"}
	_ = fs.MkdirAll(ctx.CacheDir, 0o755)

	st := &createConflictStore{Store: memstore.New()}
	svc := appbuildtest.New(meta, appbuildtest.WithFS(fs, ctx), appbuildtest.WithStore(st), appbuildtest.WithACL(acl.NopACL{}))
	rebindApp(app, fs, ctx, svc)
	app.broker = newEventBroker()
	app.SetPrincipalResolver(func(*http.Request) principal.Principal {
		return principal.Principal{User: "peer", Tool: principal.ToolSync}
	})

	// No If-Match: a first-create push. The precondition passes because
	// GetEntity reports the id absent; the conflict only surfaces on the
	// durable CreateEntity inside ApplyEntity.
	body := syncEntityBody{ID: "NOTE-1", Type: "note", Properties: map[string]any{"title": "loser"}}
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	r := httptest.NewRequest(http.MethodPut, "/api/sync/entities/NOTE-1", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(w, r)

	if w.Code != http.StatusConflict {
		t.Fatalf("create-conflict sync PUT returned %d, want 409: %s", w.Code, w.Body.String())
	}
	var errResp struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if errResp.Type == "" {
		t.Errorf("expected an error type in the 409 body; got %s", w.Body.String())
	}
}

// TestWriteSyncApplyError_CreateConflictMaps409 is the direct unit test of the
// error->status mapping: ErrEntityAlreadyExists and ErrRelationAlreadyExists
// map to 409, distinct from the 422 validation/type-immutable rejects and the
// 412 precondition. Guards the mapping independently of the full handler wiring.
func TestWriteSyncApplyError_CreateConflictMaps409(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"entity create conflict", entitymanager.ErrEntityAlreadyExists, http.StatusConflict},
		{"relation create conflict", entitymanager.ErrRelationAlreadyExists, http.StatusConflict},
		{"relation vanished on update", entitymanager.ErrRelationNotFound, http.StatusPreconditionFailed},
		// Entity vanished-on-update wraps ErrEntityNotFound but must map to 412
		// (symmetric with the relation case), NOT the 404 reserved for a missing
		// relation endpoint. The order of the errors.Is checks in
		// writeSyncApplyError is load-bearing for this — pin it here.
		{"entity vanished on update", entitymanager.ErrEntityVanishedOnUpdate, http.StatusPreconditionFailed},
		{"type immutable", entitymanager.ErrTypeImmutable, http.StatusUnprocessableEntity},
		{"endpoint missing", entitymanager.ErrEntityNotFound, http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPut, "/api/sync/entities/X", http.NoBody)
			writeSyncApplyError(w, r, tc.err)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d (body: %s)", w.Code, tc.want, w.Body.String())
			}
		})
	}
}
