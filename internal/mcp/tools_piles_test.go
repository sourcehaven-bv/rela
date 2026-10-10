package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// fakePiles is an in-memory piles capability holding one pile, "Mine"
// (PIL-AAAA), whose items include an id that does not exist.
type fakePiles struct {
	mu      sync.Mutex
	items   []entity.Ref
	pushes  []PilePush
	removed []entity.Ref
}

func newFakePiles(t *testing.T) (*fakePiles, *PileFuncs) {
	t.Helper()
	f := &fakePiles{}
	for _, a := range []string{"REQ-002", "GONE-1", "REQ-001"} {
		ref, err := entity.ParseRef(a)
		if err != nil {
			t.Fatal(err)
		}
		f.items = append(f.items, ref)
	}
	mine := func() PileInfo {
		return PileInfo{ID: "PIL-AAAA", Name: "Mine", Icon: "star", Items: append([]entity.Ref(nil), f.items...)}
	}
	pf, err := NewPileFuncs(PileFuncs{
		List: func(context.Context) ([]PileInfo, error) {
			f.mu.Lock()
			defer f.mu.Unlock()
			return []PileInfo{mine()}, nil
		},
		Find: func(_ context.Context, nameOrID string) (PileInfo, error) {
			f.mu.Lock()
			defer f.mu.Unlock()
			if nameOrID != "Mine" && nameOrID != "PIL-AAAA" {
				return PileInfo{}, errors.New("piles: pile not found")
			}
			return mine(), nil
		},
		Push: func(_ context.Context, p PilePush) (int, error) {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.pushes = append(f.pushes, p)
			return len(p.Refs), nil
		},
		Remove: func(_ context.Context, _ string, refs []entity.Ref) error {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.removed = append(f.removed, refs...)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("NewPileFuncs: %v", err)
	}
	return f, pf
}

func newPileServer(t *testing.T) (*Server, *fakePiles) {
	t.Helper()
	meta, st := makeTestFixture(t)
	f, pf := newFakePiles(t)
	srv, err := NewServer(newTestDeps(t, meta, st), "test",
		WithPrincipal(principal.Principal{User: "tester", Tool: principal.ToolMCP}), WithPiles(pf, pf))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return srv, f
}

func TestPiles_ListCountsReadableItems(t *testing.T) {
	t.Parallel()
	s, _ := newPileServer(t)
	text, isErr := callTool(t, s, "list_piles", `{}`)
	if isErr {
		t.Fatalf("list_piles: %s", text)
	}
	var out struct {
		Piles []pileSummaryJSON `json:"piles"`
	}
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Piles) != 1 || out.Piles[0].Count != 2 || out.Piles[0].Name != "Mine" {
		t.Fatalf("piles = %+v, want Mine with count 2", out.Piles)
	}
}

func TestPiles_ShowLeavesOutUnreadableItems(t *testing.T) {
	t.Parallel()
	s, _ := newPileServer(t)
	for _, pile := range []string{"Mine", "PIL-AAAA"} {
		text, isErr := callTool(t, s, "show_pile", `{"pile":"`+pile+`"}`)
		if isErr {
			t.Fatalf("show_pile: %s", text)
		}
		var out pileJSON
		if err := json.Unmarshal([]byte(text), &out); err != nil {
			t.Fatalf("decode: %v", err)
		}
		var got []string
		for _, it := range out.Items {
			got = append(got, it.Address)
		}
		if strings.Join(got, ",") != "REQ-002,REQ-001" || out.Count != 2 {
			t.Errorf("%s: items = %v (count %d), want REQ-002,REQ-001", pile, got, out.Count)
		}
		if strings.Contains(text, "GONE-1") {
			t.Errorf("%s: an unreadable item leaked: %s", pile, text)
		}
	}
	if text, isErr := callTool(t, s, "show_pile", `{"pile":"Other"}`); !isErr {
		t.Errorf("unknown pile answered %s", text)
	}
}

func TestPiles_AddResolvesAndPushes(t *testing.T) {
	t.Parallel()
	s, f := newPileServer(t)
	text, isErr := callTool(t, s, "add_to_pile",
		`{"pile":"Inbox","ids":["REQ-001","DEC-001"],"owner":"PER-1","create":false}`)
	if isErr {
		t.Fatalf("add_to_pile: %s", text)
	}
	if text != `{"added":2}` && !strings.Contains(text, `"added": 2`) {
		t.Errorf("result = %s, want added 2", text)
	}
	if len(f.pushes) != 1 {
		t.Fatalf("pushes = %v", f.pushes)
	}
	p := f.pushes[0]
	if p.Owner != "PER-1" || p.Pile != "Inbox" || p.Create || len(p.Refs) != 2 || p.Refs[0].ID != "REQ-001" {
		t.Errorf("push = %+v", p)
	}
}

func TestPiles_AddRefusesUnknownIDWithoutPushing(t *testing.T) {
	t.Parallel()
	s, f := newPileServer(t)
	text, isErr := callTool(t, s, "add_to_pile", `{"pile":"Inbox","ids":["REQ-001","NOPE-9"]}`)
	if !isErr || !strings.Contains(text, "entity not found: NOPE-9") {
		t.Fatalf("result = %s (error %v), want entity not found", text, isErr)
	}
	if len(f.pushes) != 0 {
		t.Errorf("a failed add pushed %v", f.pushes)
	}
	if text, isErr := callTool(t, s, "add_to_pile", `{"pile":"Inbox","ids":[]}`); !isErr {
		t.Errorf("empty ids answered %s", text)
	}
}

func TestPiles_RemoveMatchesPileItems(t *testing.T) {
	t.Parallel()
	s, f := newPileServer(t)
	text, isErr := callTool(t, s, "remove_from_pile", `{"pile":"Mine","ids":["REQ-001","NEVER-1"]}`)
	if isErr {
		t.Fatalf("remove_from_pile: %s", text)
	}
	if len(f.removed) != 1 || f.removed[0].ID != "REQ-001" {
		t.Errorf("removed = %v, want REQ-001 only", f.removed)
	}
	// The answer does not depend on which ids were on the pile.
	other, _ := callTool(t, s, "remove_from_pile", `{"pile":"Mine","ids":["NEVER-1"]}`)
	if other != text {
		t.Errorf("answers differ: %q vs %q", text, other)
	}
}

func TestPiles_ToolsAreOptIn(t *testing.T) {
	t.Parallel()
	meta, st := makeTestFixture(t)
	srv, err := NewServer(newTestDeps(t, meta, st), "test",
		WithPrincipal(principal.Principal{User: "tester", Tool: principal.ToolMCP}))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	result, rpcErr := dispatch(t, srv, "tools/list", `{}`)
	if rpcErr != nil {
		t.Fatalf("tools/list: %s", rpcErr.Message)
	}
	if strings.Contains(string(result), "pile") {
		t.Error("pile tools registered without WithPiles")
	}
	_, pf := newFakePiles(t)
	if _, err := NewServer(newTestDeps(t, meta, st), "test",
		WithPrincipal(principal.Principal{User: "tester", Tool: principal.ToolMCP}), WithPiles(pf, nil)); err == nil {
		t.Error("NewServer accepted WithPiles without a writer")
	}
	if _, err := NewPileFuncs(PileFuncs{}); err == nil {
		t.Error("NewPileFuncs accepted missing functions")
	}
}

// failingHeaderReader fails the batched header read, as a store fault would.
type failingHeaderReader struct{ GraphReader }

func (failingHeaderReader) ResolveHeadersErr(
	context.Context, []entity.Ref,
) (map[entity.Ref]visibility.ResolvedHeader, error) {
	return nil, errors.New("header read failed")
}

// A failed header read is a tool error: never an empty pile, never a
// missing entity.
func TestPiles_HeaderReadFailureIsToolError(t *testing.T) {
	t.Parallel()
	meta, st := makeTestFixture(t)
	f, pf := newFakePiles(t)
	deps := newTestDeps(t, meta, st)
	deps.Store = failingHeaderReader{GraphReader: deps.Store}
	srv, err := NewServer(deps, "test",
		WithPrincipal(principal.Principal{User: "tester", Tool: principal.ToolMCP}), WithPiles(pf, pf))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	for _, tc := range []struct{ tool, args string }{
		{"list_piles", `{}`},
		{"show_pile", `{"pile":"Mine"}`},
		{"add_to_pile", `{"pile":"Mine","ids":["REQ-001"]}`},
	} {
		text, isErr := callTool(t, srv, tc.tool, tc.args)
		if !isErr || !strings.Contains(text, "reading pile items failed") {
			t.Errorf("%s: result = %s (error %v), want the read failure", tc.tool, text, isErr)
		}
	}
	if len(f.pushes) != 0 {
		t.Errorf("a failed read pushed %v", f.pushes)
	}
}

// add_to_pile resolves its ids with the same number of store reads
// whatever their number: one batch, never a read per id.
func TestPiles_AddReadBudget(t *testing.T) {
	t.Parallel()
	reads := make([]int, 0, 2)
	for _, n := range []int{10, 50} {
		meta, st := makeTestFixture(t)
		ids := make([]string, n)
		for i := range ids {
			ids[i] = fmt.Sprintf("REQ-%04d", 100+i)
			e := &entity.Entity{ID: ids[i], Type: "requirement", Properties: map[string]any{"title": ids[i]}}
			if err := st.CreateEntity(context.Background(), e); err != nil {
				t.Fatal(err)
			}
		}
		counting := storetest.NewCounting(st)
		f, pf := newFakePiles(t)
		srv, err := NewServer(newTestDeps(t, meta, counting), "test",
			WithPrincipal(principal.Principal{User: "tester", Tool: principal.ToolMCP}), WithPiles(pf, pf))
		if err != nil {
			t.Fatalf("NewServer: %v", err)
		}
		body, _ := json.Marshal(map[string]any{"pile": "Inbox", "ids": ids})
		counting.Reset()
		if text, isErr := callTool(t, srv, "add_to_pile", string(body)); isErr {
			t.Fatalf("add_to_pile: %s", text)
		}
		if len(f.pushes) != 1 || len(f.pushes[0].Refs) != n {
			t.Fatalf("pushes = %v, want one push of %d refs", f.pushes, n)
		}
		reads = append(reads, counting.Reads())
	}
	if reads[0] != reads[1] {
		t.Errorf("reads grew with the id count: %d at 10, %d at 50", reads[0], reads[1])
	}
}
