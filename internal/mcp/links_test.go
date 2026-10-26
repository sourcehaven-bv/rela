package mcp

import (
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
)

const testBase = "https://rela.example.com"

func TestEntityURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		base bool
		typ  string
		ref  entity.Ref
		want string
	}{
		{"implicit face", true, "requirement", entity.Ref{ID: "REQ-1"}, testBase + "/entity/requirement/REQ-1"},
		{"faced row", true, "solution", entity.Ref{ID: "SOL-1", Face: "draft"}, testBase + "/entity/solution/SOL-1@draft"},
		{"escapes path segments", true, "a b", entity.Ref{ID: "X/1"}, testBase + "/entity/a%20b/X%2F1"},
		{"no base", false, "requirement", entity.Ref{ID: "REQ-1"}, ""},
		{"no type", true, "", entity.Ref{ID: "REQ-1"}, ""},
		{"no id", true, "requirement", entity.Ref{}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			base := ""
			if tc.base {
				base = testBase
			}
			ctx := withBaseURL(t.Context(), base)
			if got := entityURL(ctx, tc.typ, tc.ref); got != tc.want {
				t.Errorf("entityURL = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFamilyURL(t *testing.T) {
	t.Parallel()
	ctx := withBaseURL(t.Context(), testBase)
	if got, want := familyURL(ctx, "solution", "SOL-1"), testBase+"/entity/solution/SOL-1"; got != want {
		t.Errorf("familyURL = %q, want %q", got, want)
	}
	if got := familyURL(ctx, "solution", ""); got != "" {
		t.Errorf("familyURL without id = %q, want empty", got)
	}
	if got := familyURL(t.Context(), "solution", "SOL-1"); got != "" {
		t.Errorf("familyURL without base = %q, want empty", got)
	}
}

func TestRequestBaseURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		host  string
		proto string
		tls   bool
		want  string
	}{
		{"plain http", "rela.example.com", "", false, "http://rela.example.com"},
		{"forwarded https", "rela.example.com", "HTTPS", false, "https://rela.example.com"},
		{"tls", "rela.example.com:8443", "", true, "https://rela.example.com:8443"},
		{"empty host", "", "", false, ""},
		{"unsafe host", "evil.com/x?", "", false, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := httptest.NewRequest(http.MethodPost, "/api/v1/_mcp", http.NoBody)
			r.Host = tc.host
			if tc.proto != "" {
				r.Header.Set("X-Forwarded-Proto", tc.proto)
			}
			if tc.tls {
				r.TLS = &tls.ConnectionState{}
			}
			if got := requestBaseURL(r); got != tc.want {
				t.Errorf("requestBaseURL = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestEntityResultsCarryURL pins the url on every entity-shaped result built
// in convert.go, including face refs, and its absence without a base.
func TestEntityResultsCarryURL(t *testing.T) {
	t.Parallel()
	meta := testMeta()
	meta.Relations["cites"] = metamodel.RelationDef{
		Label: "Cites", From: []string{"solution"}, To: []string{"requirement"}, Scope: metamodel.ScopeContent,
	}
	st := memstore.New()
	draft := &entity.Entity{ID: "SOL-1", Type: "solution", Face: "draft", Properties: map[string]any{"title": "D"}}
	req := &entity.Entity{ID: "REQ-1", Type: "requirement", Properties: map[string]any{"title": "R"}}
	seedEntity(t, st, draft)
	seedEntity(t, st, req)
	key := entity.RelationKey{From: "SOL-1", FromFace: "draft", Type: "cites", To: "REQ-1"}
	if _, err := st.CreateRelation(t.Context(), key, &store.RelationData{}); err != nil {
		t.Fatal(err)
	}
	ctx := withBaseURL(t.Context(), testBase)
	solURL := testBase + "/entity/solution/SOL-1@draft"
	reqURL := testBase + "/entity/requirement/REQ-1"

	t.Run("entity", func(t *testing.T) {
		t.Parallel()
		ej := buildEntityJSON(ctx, draft, graphOf(st), meta, entityView{relations: true})
		if ej.URL != solURL {
			t.Errorf("url = %q, want %q", ej.URL, solURL)
		}
		if got := ej.Relations.Outgoing["cites"]; len(got) != 1 || got[0].URL != reqURL {
			t.Errorf("outgoing cites = %+v, want url %q", got, reqURL)
		}
	})
	t.Run("incoming relation names the tail face", func(t *testing.T) {
		t.Parallel()
		ej := buildEntityJSON(ctx, req, graphOf(st), meta, entityView{relations: true})
		if got := ej.Relations.Incoming["cites"]; len(got) != 1 || got[0].URL != solURL {
			t.Errorf("incoming cites = %+v, want url %q", got, solURL)
		}
	})
	t.Run("summary", func(t *testing.T) {
		t.Parallel()
		if got := convertStoreEntitySummary(ctx, meta, draft).URL; got != solURL {
			t.Errorf("summary url = %q, want %q", got, solURL)
		}
	})
	t.Run("trace", func(t *testing.T) {
		t.Parallel()
		tr := &tracer.TraceResult{ID: "REQ-1", Type: "requirement"}
		if got := convertTraceNode(ctx, tr, meta).URL; got != reqURL {
			t.Errorf("trace url = %q, want %q", got, reqURL)
		}
	})
	t.Run("no base omits the field", func(t *testing.T) {
		t.Parallel()
		ej := buildEntityJSON(t.Context(), req, graphOf(st), meta, entityView{relations: true})
		text, err := marshalJSON(ej)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(text, `"url"`) {
			t.Errorf("entity = %s, want no url", text)
		}
	})
}

// TestHTTPHandlerEntityURL pins that the HTTP transport derives the base
// from the request, so a tool result links to the public host.
func TestHTTPHandlerEntityURL(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(newDispatchServer(t).HTTPHandler())
	t.Cleanup(ts.Close)
	const origin = "https://atlas.example.internal"
	reqURL := origin + "/entity/requirement/REQ-001"
	decURL := origin + "/entity/decision/DEC-001"

	t.Run("show_entity", func(t *testing.T) {
		t.Parallel()
		text := callToolHTTP(t, ts, "show_entity", `{"id":"REQ-001"}`)
		var ej entityJSON
		if err := json.Unmarshal([]byte(text), &ej); err != nil {
			t.Fatalf("parse %s: %v", text, err)
		}
		if ej.URL != reqURL {
			t.Errorf("url = %q, want %q", ej.URL, reqURL)
		}
		if got := ej.Relations.Incoming["addresses"]; len(got) != 1 || got[0].URL != decURL {
			t.Errorf("incoming addresses = %+v, want url %q", got, decURL)
		}
	})
	t.Run("find_path names the resolved row", func(t *testing.T) {
		t.Parallel()
		text := callToolHTTP(t, ts, "find_path", `{"from":"DEC-001","to":"REQ-001"}`)
		var steps []pathStepJSON
		if err := json.Unmarshal([]byte(text), &steps); err != nil {
			t.Fatalf("parse %s: %v", text, err)
		}
		if len(steps) != 2 || steps[0].URL != decURL || steps[1].URL != reqURL {
			t.Errorf("path = %+v, want urls %q, %q", steps, decURL, reqURL)
		}
	})
}

// callToolHTTP posts one tools/call of name with the JSON args to ts, as a
// proxy forwarding the public Host over https would, and returns the text
// result.
func callToolHTTP(t *testing.T, ts *httptest.Server, name, args string) string {
	t.Helper()
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call",` +
		`"params":{"name":"` + name + `","arguments":` + args + `}}`
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, ts.URL, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "atlas.example.internal"
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, raw)
	}
	return toolText(t, raw)
}

// toolText returns the first text content of a tools/call response, which
// the stateless transport sends as JSON or as one SSE data line.
func toolText(t *testing.T, raw []byte) string {
	t.Helper()
	payload := string(raw)
	for line := range strings.SplitSeq(payload, "\n") {
		if rest, ok := strings.CutPrefix(line, "data: "); ok {
			payload = rest
			break
		}
	}
	var msg struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(payload), &msg); err != nil {
		t.Fatalf("parse response %s: %v", raw, err)
	}
	if msg.Result.IsError || len(msg.Result.Content) == 0 {
		t.Fatalf("tool result = %s", raw)
	}
	return msg.Result.Content[0].Text
}
