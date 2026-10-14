package subscription

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"maps"
	"os"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/ingest"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// world is an in-memory graph serving ingest's reader, writer and write-prep
// reads, counting each.
type world struct {
	rows      []*entity.Entity
	createErr func(e *entity.Entity) error
	lists     int
	creates   int
	patches   int
	nextID    int
}

func (w *world) GetAddress(context.Context, string) (*entity.Entity, error) {
	return nil, store.ErrNotFound
}

func (w *world) ListEntities(_ context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	w.lists++
	return func(yield func(*entity.Entity, error) bool) {
		for _, e := range w.rows {
			if e.Type == q.Type && !yield(e, nil) {
				return
			}
		}
	}
}

func (w *world) GetEntity(_ context.Context, ref entity.Ref) (*entity.Entity, error) {
	for _, e := range w.rows {
		if e.Ref() == ref {
			return e, nil
		}
	}
	return nil, store.ErrNotFound
}

func (w *world) CreateEntity(
	_ context.Context, e *entity.Entity, _ entity.CreateOptions,
) (*entity.CreateResult, error) {
	if w.createErr != nil {
		if err := w.createErr(e); err != nil {
			return nil, err
		}
	}
	w.creates++
	w.nextID++
	e.ID = fmt.Sprintf("A-%d", w.nextID)
	w.rows = append(w.rows, e)
	return &entity.CreateResult{Entity: e}, nil
}

func (w *world) PatchEntity(_ context.Context, id string, p entity.Patch) (*entity.UpdateResult, error) {
	w.patches++
	for _, e := range w.rows {
		if e.ID == id {
			maps.Copy(e.Properties, p.Properties)
		}
	}
	return &entity.UpdateResult{}, nil
}

func (w *world) delete(id string) {
	for i, e := range w.rows {
		if e.ID == id {
			w.rows = append(w.rows[:i], w.rows[i+1:]...)
			return
		}
	}
}

func (w *world) titles() []string {
	out := make([]string, len(w.rows))
	for i, e := range w.rows {
		out[i], _ = e.Properties["title"].(string)
	}
	return out
}

// mapKV is an in-memory state.KV.
type mapKV struct {
	mu sync.Mutex
	m  map[string][]byte
}

func (k *mapKV) Get(_ context.Context, key string) ([]byte, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	v, ok := k.m[key]
	if !ok {
		return nil, os.ErrNotExist
	}
	return v, nil
}

func (k *mapKV) Put(_ context.Context, key string, data []byte) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.m == nil {
		k.m = map[string][]byte{}
	}
	k.m[key] = data
	return nil
}

func (k *mapKV) Delete(_ context.Context, key string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.m, key)
	return nil
}

func testDeps(w *world, kv *mapKV, secretValues map[string]string) Deps {
	return Deps{
		Ingest: ingest.Deps{Reader: w, Writer: w, WritePrep: w, MaxAttempts: 2, Label: "test"},
		State:  kv,
		Secrets: func() (map[string]string, error) {
			return secretValues, nil
		},
		Fetcher: &Fetcher{},
		Logger:  slog.New(slog.DiscardHandler),
		Now:     func() time.Time { return time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC) },
	}
}

func discardWarn(string, ...any) {}
