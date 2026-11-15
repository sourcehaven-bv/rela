package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// filteredToolNames builds a server with WithToolFilter(allow, deny) and
// returns the names tools/list serves.
func filteredToolNames(t *testing.T, allow, deny []string) (srv *Server, names []string) {
	t.Helper()
	meta, st := makeTestFixture(t)
	srv, err := NewServer(newTestDeps(t, meta, st), "test",
		WithPrincipal(principal.Principal{User: "tester", Tool: principal.ToolMCP}),
		WithToolFilter(allow, deny))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	result, rpcErr := dispatch(t, srv, "tools/list", `{}`)
	if rpcErr != nil {
		t.Fatalf("tools/list: JSON-RPC error %d: %s", rpcErr.Code, rpcErr.Message)
	}
	var decoded struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(result, &decoded); err != nil {
		t.Fatalf("decode tools/list result: %v", err)
	}
	names = make([]string, 0, len(decoded.Tools))
	for _, tool := range decoded.Tools {
		names = append(names, tool.Name)
	}
	slices.Sort(names)
	return srv, names
}

func TestWithToolFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		allow, deny []string
		want        []string // exact set, when set
		present     []string
		absent      []string
	}{
		{
			name:  "allow serves only the listed tools",
			allow: []string{"show_entity", "list_entities", "watch"},
			want:  []string{"list_entities", "show_entity", "watch"},
		},
		{
			name:    "deny removes the listed tools",
			deny:    []string{"create_entity", "delete_entity", "attach_file"},
			present: []string{"show_entity", "update_entity"},
			absent:  []string{"create_entity", "delete_entity", "attach_file"},
		},
		{
			name:  "deny wins over allow",
			allow: []string{"show_entity", "create_entity"},
			deny:  []string{"create_entity"},
			want:  []string{"show_entity"},
		},
		{
			name:    "names are trimmed",
			deny:    []string{" create_entity "},
			present: []string{"show_entity"},
			absent:  []string{"create_entity"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, got := filteredToolNames(t, tc.allow, tc.deny)
			if tc.want != nil && !slices.Equal(got, tc.want) {
				t.Errorf("tools = %v, want %v", got, tc.want)
			}
			for _, name := range tc.present {
				if !slices.Contains(got, name) {
					t.Errorf("tool %q missing from %v", name, got)
				}
			}
			for _, name := range tc.absent {
				if slices.Contains(got, name) {
					t.Errorf("tool %q served despite the filter", name)
				}
			}
		})
	}
}

// A filtered-out tool is not callable, either directly or as a watch target.
func TestWithToolFilter_FilteredToolIsUnreachable(t *testing.T) {
	t.Parallel()

	srv, _ := filteredToolNames(t, nil, []string{"show_entity"})
	if _, rpcErr := dispatch(t, srv, "tools/call",
		`{"name":"show_entity","arguments":{"id":"x"}}`); rpcErr == nil {
		t.Error("show_entity dispatched although the filter denies it")
	}
	text, isError := callTool(t, srv, "watch",
		`{"targets":[{"tool":"show_entity","args":{"id":"x"}}],"timeout":1}`)
	if !isError || !strings.Contains(text, "must be one of") || strings.Contains(text, "show_entity") {
		t.Errorf("watch on a denied tool = %q (error %v), want a refusal that does not offer it", text, isError)
	}
}

// A name the server does not register fails construction, so a misspelled
// deny entry cannot leave the tool on.
func TestWithToolFilter_UnknownNameIsRefused(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		allow, deny []string
	}{
		{name: "misspelled deny", deny: []string{"delete-entity"}},
		{name: "misspelled allow", allow: []string{"show_entities"}},
		{name: "not a rela tool", deny: []string{"exec_shell"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			meta, st := makeTestFixture(t)
			_, err := NewServer(newTestDeps(t, meta, st), "test",
				WithPrincipal(principal.Principal{User: "tester", Tool: principal.ToolMCP}),
				WithToolFilter(tc.allow, tc.deny))
			if !errors.Is(err, errUnknownFilteredTool) {
				t.Fatalf("NewServer error = %v, want errUnknownFilteredTool", err)
			}
		})
	}
}

// A real tool whose feature is off filters nothing and is accepted, so a
// Configure save that turns the feature off cannot fail the rebuild. The
// test server has no comments, channels or actions, and no Lua tools.
func TestWithToolFilter_ToolOfAFeatureThatIsOffIsAccepted(t *testing.T) {
	t.Parallel()

	_, got := filteredToolNames(t, nil, []string{"list_comments", "run_action", "read_channel", "lua_eval"})
	if !slices.Contains(got, "show_entity") {
		t.Errorf("tools = %v, want show_entity still served", got)
	}
}

// The filter narrows tools, not read access: an entity resource still reads
// when show_entity is denied. The docs say so; this pins it.
func TestWithToolFilter_ResourcesStayReadable(t *testing.T) {
	t.Parallel()

	srv, _ := filteredToolNames(t, []string{"schema"}, nil)
	cs := connect(t, srv)
	templates, err := cs.ListResourceTemplates(context.Background(), nil)
	if err != nil {
		t.Fatalf("resources/templates/list: %v", err)
	}
	if len(templates.ResourceTemplates) == 0 {
		t.Error("no resource templates; the docs say the filter leaves resources on")
	}
}

// TestKnownToolNames_MatchesRegistrations pins knownToolNames to the package's newTool
// calls: a new tool missing from the catalog would be refused in a filter,
// and a removed tool left in it would be accepted.
func TestKnownToolNames_MatchesRegistrations(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	consts := map[string]string{}
	var args []ast.Expr
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.ValueSpec:
				for i, v := range n.Values {
					if lit, ok := v.(*ast.BasicLit); ok && lit.Kind == token.STRING {
						consts[n.Names[i].Name], _ = strconv.Unquote(lit.Value)
					}
				}
			case *ast.CallExpr:
				if id, ok := n.Fun.(*ast.Ident); ok && id.Name == "newTool" && len(n.Args) > 0 {
					args = append(args, n.Args[0])
				}
			}
			return true
		})
	}
	registered := map[string]bool{}
	for _, arg := range args {
		switch a := arg.(type) {
		case *ast.BasicLit:
			name, _ := strconv.Unquote(a.Value)
			registered[name] = true
		case *ast.Ident:
			name, ok := consts[a.Name]
			if !ok {
				t.Fatalf("newTool(%s): not a string constant", a.Name)
			}
			registered[name] = true
		default:
			t.Fatalf("newTool called with %T; give it a literal or a constant", arg)
		}
	}
	if len(registered) == 0 {
		t.Fatal("found no newTool calls")
	}
	for name := range registered {
		if !knownToolNames[name] {
			t.Errorf("tool %q is registered but missing from knownToolNames", name)
		}
	}
	for name := range knownToolNames {
		if !registered[name] {
			t.Errorf("knownToolNames has %q, which no newTool call registers", name)
		}
	}
}
