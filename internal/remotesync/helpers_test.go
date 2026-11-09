package remotesync

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/canonical"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/lock"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/reconcile"
	"github.com/Sourcehaven-BV/rela/internal/reconclient/httpremote"
	"github.com/Sourcehaven-BV/rela/internal/reconwire"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/templating"
)

const testSchema = `
version: "1"
entities:
  requirement:
    label: Requirement
    id_prefix: REQ
    properties:
      title: {type: string}
      status: {type: enum, values: [open, done]}
      note: {type: string}
      cat: {type: external_ref, system: catalog}
  component:
    label: Component
    id_prefix: CMP
    properties:
      name: {type: string}
      cat: {type: external_ref, system: catalog}
  other:
    label: Other
    id_prefix: OTH
    properties:
      name: {type: string}
      ref: {type: external_ref, system: elsewhere}
relations:
  satisfies:
    label: Satisfies
    from: [requirement]
    to: [component]
    properties:
      weight: {type: integer, min: 0, max: 10}
  depends:
    label: Depends
    from: [component]
    to: [component]
`

const testRemotes = `
remotes:
  cat:
    url: https://catalog.example
    credential: {secret: remote_credential.cat}
    entities:
      req:
        type: requirement
        ref: cat
        fields: {title: title, status: state, content: content}
      component:
        ref: cat
        fields: {name: name}
    relations:
      satisfies:
        fields: {weight: weight}
      depends: {}
`

func testMeta(t *testing.T) *metamodel.Metamodel {
	t.Helper()
	meta, err := metamodel.Parse([]byte(testSchema))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	return meta
}

func testMapping(t *testing.T, meta *metamodel.Metamodel, edit func(p *Remote)) *Mapping {
	t.Helper()
	cfg, err := Parse([]byte(testRemotes), meta)
	if err != nil {
		t.Fatalf("parse remotes: %v", err)
	}
	p := cfg.Remotes["cat"]
	if edit != nil {
		edit(&p)
	}
	m, err := Compile("cat", p, meta)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return m
}

// fakeEnt is one source entity.
type fakeEnt struct {
	Type, ID   string
	Props      map[string]any
	Content    string
	Redacted   []string
	noRedacted bool
}

// fakeRel is one source relation.
type fakeRel struct {
	From, Type, To string
	Meta           map[string]any
}

// fakeSource is a source instance in memory: it answers reconcile requests
// with the server's semantics and serves the read routes.
type fakeSource struct {
	mu   sync.Mutex
	ents map[string]*fakeEnt
	rels map[string]*fakeRel
	// deny answers every read route 403.
	deny bool
	// answer rewrites a read route's body.
	answer func(path string, body []byte) []byte
	gets   []string
}

func newFakeSource() *fakeSource {
	return &fakeSource{ents: map[string]*fakeEnt{}, rels: map[string]*fakeRel{}}
}

func (f *fakeSource) put(e *fakeEnt) { f.ents[e.ID] = e }

func (f *fakeSource) relate(from, typ, to string, meta map[string]any) {
	f.rels[from+"--"+typ+"--"+to] = &fakeRel{From: from, Type: typ, To: to, Meta: meta}
}

func digestOf(v any) canonical.Digest {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return sha256.Sum256(b)
}

func (f *fakeSource) elements() []reconcile.Element {
	var out []reconcile.Element
	for _, e := range f.ents {
		out = append(out, reconcile.Element{Kind: reconcile.KindEntity, Address: e.Type + "/" + e.ID,
			Hash: digestOf([]any{e.Props, e.Content})})
	}
	for k, r := range f.rels {
		out = append(out, reconcile.Element{Kind: reconcile.KindRelation, Address: k, Hash: digestOf(r.Meta)})
	}
	slices.SortFunc(out, func(a, b reconcile.Element) int {
		return cmp.Or(cmp.Compare(a.Kind, b.Kind), cmp.Compare(a.Address, b.Address))
	})
	return out
}

func (f *fakeSource) Reconcile(_ context.Context, req reconwire.Request) (reconwire.Response, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.deny {
		return reconwire.Response{}, fmt.Errorf("%w: answered 403", httpremote.ErrDenied)
	}
	els := f.elements()
	var out reconwire.Response
	for _, rr := range req.Ranges {
		kind, names := reconcile.KindEntity, rr.Entities
		if rr.Kind == "relation" {
			kind, names = reconcile.KindRelation, rr.Relations
		}
		lo := rr.Lo
		if rr.After != nil {
			lo = *rr.After + "\x01"
		}
		var in []reconcile.Element
		for _, e := range els {
			typ, _, _ := strings.Cut(e.Address, "/")
			if e.Kind == reconcile.KindRelation {
				typ = strings.Split(e.Address, "--")[1]
			}
			if e.Kind == kind && slices.Contains(names, typ) && e.Address >= lo && (rr.Hi == nil || e.Address < *rr.Hi) {
				in = append(in, e)
			}
		}
		if rr.Op == "fingerprint" {
			var rg reconcile.Range
			for _, e := range in {
				rg = rg.Add(e)
			}
			n := rg.Count
			out.Ranges = append(out.Ranges, reconwire.Result{Status: reconwire.StatusOK, Count: &n,
				Fingerprint: rg.Sum.String()})
			continue
		}
		res := reconwire.Result{Status: reconwire.StatusOK}
		if len(in) > rr.Limit {
			in, res.More = in[:rr.Limit], true
		}
		for _, e := range in {
			res.Elements = append(res.Elements, reconwire.Element{Address: e.Address, Hash: e.Hash.String()})
		}
		if len(in) > 0 {
			res.Last = in[len(in)-1].Address
		}
		out.Ranges = append(out.Ranges, res)
	}
	return out, nil
}

func (f *fakeSource) Get(_ context.Context, path string, _ int64) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gets = append(f.gets, path)
	if f.deny {
		return nil, fmt.Errorf("%w: answered 403", httpremote.ErrDenied)
	}
	body, err := f.route(path)
	if err != nil {
		return nil, err
	}
	if f.answer != nil {
		body = f.answer(path, body)
	}
	return body, nil
}

func (f *fakeSource) route(path string) ([]byte, error) {
	if path == "_schema" {
		return json.Marshal(map[string]any{"entities": map[string]any{
			"req": map[string]any{"plural": "reqs"}, "component": map[string]any{"plural": "components"},
		}})
	}
	segs := strings.Split(path, "/")
	for i, s := range segs {
		u, err := url.PathUnescape(s)
		if err != nil {
			return nil, err
		}
		segs[i] = u
	}
	switch len(segs) {
	case 2:
		e, ok := f.ents[segs[1]]
		if !ok {
			return nil, fmt.Errorf("%w: fake", httpremote.ErrNotFound)
		}
		out := map[string]any{"id": e.ID, "type": e.Type, "properties": e.Props}
		if e.Content != "" {
			out["content"] = e.Content
		}
		if !e.noRedacted {
			out["_redacted"] = append([]string{}, e.Redacted...)
		}
		return json.Marshal(out)
	case 5:
		r, ok := f.rels[segs[1]+"--"+segs[3]+"--"+segs[4]]
		if !ok {
			return nil, fmt.Errorf("%w: fake", httpremote.ErrNotFound)
		}
		return json.Marshal(map[string]any{"from": r.From, "type": r.Type, "to": r.To, "meta": r.Meta,
			"_redacted": []string{}})
	}
	return nil, fmt.Errorf("%w: fake route %s", httpremote.ErrNotFound, path)
}

// fetches counts the entity and relation reads, not the schema.
func (f *fakeSource) fetches() int {
	n := 0
	for _, g := range f.gets {
		if g != "_schema" {
			n++
		}
	}
	return n
}

type nopTemplater struct{}

func (nopTemplater) EntityTemplate(context.Context, string, string) (*templating.Template, error) {
	return nil, nil //nolint:nilnil // no template
}

func (nopTemplater) RelationTemplate(context.Context, string) (*templating.Template, error) {
	return nil, nil //nolint:nilnil // no template
}

// target is the local instance.
type target struct {
	st   store.Store
	mgr  *entitymanager.Manager
	kv   *mapKV
	meta *metamodel.Metamodel
}

func newTarget(t *testing.T) *target {
	t.Helper()
	meta := testMeta(t)
	st := memstore.New()
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: st, Meta: meta, Templater: nopTemplater{}, Audit: audit.Nop{}, ACL: acl.NopACL{},
		Transitions: statemachine.EmptySet(), FieldGate: entitymanager.AllowAllFieldGate{},
		AttachmentLocker: lock.NewMemoryLocker(), AnchorGuard: entitymanager.NopAnchorGuard{},
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	return &target{st: st, mgr: mgr, kv: &mapKV{}, meta: meta}
}

func (tg *target) deps(src Source) Deps {
	return Deps{
		Source: src, Meta: tg.meta, Writer: tg.mgr, Store: tg.st,
		State: tg.kv, Logger: slog.New(slog.DiscardHandler), RunID: "run-1",
	}
}

// byRef returns the local entity holding source id in its ref, or nil.
func (tg *target) byRef(t *testing.T, id string) *entity.Entity {
	t.Helper()
	var out *entity.Entity
	for _, typ := range []string{"requirement", "component"} {
		for e, err := range tg.st.ListEntities(context.Background(), store.EntityQuery{Type: typ, Faces: store.AllFaces()}) {
			if err != nil {
				t.Fatal(err)
			}
			if got, _ := metamodel.ExternalRefID(e.Properties["cat"]); got == id {
				out = e
			}
		}
	}
	return out
}

func (tg *target) count(t *testing.T, typ string) int {
	t.Helper()
	n := 0
	for _, err := range tg.st.ListEntities(context.Background(), store.EntityQuery{Type: typ, Faces: store.AllFaces()}) {
		if err != nil {
			t.Fatal(err)
		}
		n++
	}
	return n
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
	k.m[key] = slices.Clone(data)
	return nil
}

func (k *mapKV) Delete(_ context.Context, key string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.m, key)
	return nil
}
