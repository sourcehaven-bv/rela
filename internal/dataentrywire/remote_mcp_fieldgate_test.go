//go:build !postgres

package dataentrywire

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/attachment"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

const fieldGateMetamodel = `version: "1.0"
entities:
  ticket:
    label: Ticket
    id_prefix: "TKT-"
    id_type: sequential
    properties:
      title:
        type: string
      note:
        type: string
relations: {}
`

// alice may update tickets, but her `fields:` block names only title, so
// note is read-only for her.
const fieldGatePolicy = `roles:
  editor:
    read: [ticket]
    update: [ticket]
    fields:
      ticket:
        - field: title
assignments:
  alice: editor
`

// TestRemoteMCP_UpdateEntityHonorsFieldGrants (BUG-VUNO11) drives
// update_entity through the server rela-server mounts, built by appbuild.New
// rather than a test fixture. A write to a field the caller's `fields:` grant
// leaves out is refused and leaves the stored value alone; a write to a
// granted field succeeds.
func TestRemoteMCP_UpdateEntityHonorsFieldGrants(t *testing.T) {
	svc := fieldGateMCPServices(t)
	seed := entity.New("TKT-001", "ticket")
	seed.SetString("title", "Charlie")
	seed.SetString("note", "original")
	if err := svc.Store().CreateEntity(context.Background(), seed); err != nil {
		t.Fatalf("seed: %v", err)
	}
	cs := aliceMCPSession(t, svc)

	denied := callUpdate(t, cs, map[string]any{"note": "edited"})
	if !denied.IsError {
		t.Fatalf("update_entity wrote note, which alice's field grant leaves out: %s", toolText(denied))
	}
	if !strings.Contains(toolText(denied), "field-affordance:read-only:note") {
		t.Errorf("refusal does not name the field rule: %s", toolText(denied))
	}
	got, err := svc.Store().GetEntity(context.Background(), seed.Ref())
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if note := got.GetString("note"); note != "original" {
		t.Errorf("note = %q after a refused write, want %q", note, "original")
	}

	allowed := callUpdate(t, cs, map[string]any{"title": "Delta"})
	if allowed.IsError {
		t.Fatalf("update_entity refused title, which alice may write: %s", toolText(allowed))
	}
	got, err = svc.Store().GetEntity(context.Background(), seed.Ref())
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if title, note := got.GetString("title"), got.GetString("note"); title != "Delta" || note != "original" {
		t.Errorf("after the allowed write title = %q and note = %q, want %q and %q", title, note, "Delta", "original")
	}
}

func fieldGateMCPServices(t *testing.T) *appbuild.Services {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"entities", "relations", filepath.Join(".rela", "audit")} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range map[string]string{project.SchemaFile: fieldGateMetamodel, "acl.yaml": fieldGatePolicy} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	fs := storage.NewSafeFS(storage.NewOsFS())
	paths, err := project.Discover(root, fs)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	svc, err := appbuild.New(appbuild.Config{
		FS: fs, Paths: paths, ScriptEngine: script.NewEngine(), Audit: audit.Nop{},
	})
	if err != nil {
		t.Fatalf("appbuild.New: %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	return svc
}

// aliceMCPSession serves the production remote MCP server over HTTP with
// every request bound to alice, as requireVerifiedJWT and attachACLRequest
// bind a verified caller, and returns a connected client session.
func aliceMCPSession(t *testing.T, svc *appbuild.Services) *mcpgo.ClientSession {
	t.Helper()
	host := testHost(t, svc)
	host.AttachmentUploads = attachment.NewLimiter(attachment.DefaultMaxUploads)
	srv, err := RemoteMCPServer(svc, host, "test")
	if err != nil {
		t.Fatalf("RemoteMCPServer: %v", err)
	}
	inner := srv.HTTPHandler()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inner.ServeHTTP(w, r.WithContext(boundCtxFrom(r.Context(), t, svc, "alice")))
	}))
	t.Cleanup(ts.Close)

	client := mcpgo.NewClient(&mcpgo.Implementation{Name: "test", Version: "0"}, nil)
	cs, err := client.Connect(context.Background(), &mcpgo.StreamableClientTransport{
		Endpoint: ts.URL, DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func callUpdate(t *testing.T, cs *mcpgo.ClientSession, props map[string]any) *mcpgo.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcpgo.CallToolParams{
		Name: "update_entity", Arguments: map[string]any{"id": "TKT-001", "properties": props},
	})
	if err != nil {
		t.Fatalf("update_entity: %v", err)
	}
	return res
}

func toolText(res *mcpgo.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcpgo.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}
