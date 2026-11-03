package dataentry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/acl/acltest"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/appbuildtest"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// attachApp is an App over the audit fixture of rela.attach_render
// (TKT-1TS55R) with real action and document scripts on disk.
type attachApp struct {
	app   *App
	decl  *acl.Declarative
	store store.Store
	audit *audit.Memory
	root  string
	seq   atomic.Int64
}

// attachMeta: an audit with file properties, and findings a document and a
// list report on. "pdf" writes a PDF header before the markdown, so the
// stored bytes sniff as application/pdf like a real converter's output.
func attachMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"audit": {
				Label: "Audit", IDPrefix: "AUD-",
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: "string"},
					"report": {Type: "file", Accept: []string{"text/plain", "application/pdf"}},
					"frozen": {Type: "file", Accept: []string{"text/plain"}},
				},
				PropertyOrder: []string{"title", "report", "frozen"},
			},
			"finding": {
				Label: "Finding", IDPrefix: "FND-",
				Properties:    map[string]metamodel.PropertyDef{"title": {Type: "string"}},
				PropertyOrder: []string{"title"},
			},
		},
		Transforms: map[string]metamodel.TransformDef{
			"copy": {From: "markdown", Command: []string{"cp", "{in}", "{out}"}, Produces: "text/plain"},
			"pdf": {From: "markdown", Command: []string{
				"sh", "-c", `printf '%%PDF-1.4\n' > "$2" && cat "$1" >> "$2"`, "sh", "{in}", "{out}",
			}, Produces: "application/pdf"},
		},
	}
}

// attachPolicy: alice reads and updates audits and reads findings; carol
// does the same but cannot read findings; dave reads everything and updates
// nothing; alice alone holds reports:view.
func attachPolicy() *acl.Policy {
	return &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"auditor": {
				Read: []string{"audit", "finding"}, Update: []string{"audit"},
				Permissions: []string{"reports:view"},
			},
			"limited": {Read: []string{"audit"}, Update: []string{"audit"}},
			"reader":  {Read: []string{"audit", "finding"}},
		},
		Assignments: map[string]string{"alice": "auditor", "carol": "limited", "dave": "reader"},
	}
}

const reportScript = `
local a = rela.get_entity(rela.document.entry_id)
print("# Report for " .. a.properties.title)
for _, f in ipairs(rela.list_entities("finding")) do
  print("- " .. f.properties.title)
end
`

func newAttachApp(t *testing.T) *attachApp {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"actions", filepath.Join("scripts", "docs")} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(root, "scripts", "docs", "report.lua"), reportScript)
	writeFile(t, filepath.Join(root, "scripts", "docs", "summary.lua"), `print("summary of findings")`)

	meta := attachMeta()
	fs := storage.NewSafeFS(storage.NewOsFS())
	paths := &project.Context{Root: root, CacheDir: filepath.Join(root, ".rela")}
	st := appbuildtest.New(meta, appbuildtest.WithFS(fs, paths)).Store()
	ctx := context.Background()
	for _, e := range []*entity.Entity{
		{ID: "AUD-1", Type: "audit", Properties: map[string]any{"title": "Annual audit"}},
		{ID: "FND-1", Type: "finding", Properties: map[string]any{"title": "Weak passwords"}},
		{ID: "FND-2", Type: "finding", Properties: map[string]any{"title": "Open firewall"}},
	} {
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatalf("seed %s: %v", e.ID, err)
		}
	}

	decl := mustNewACL(t, attachPolicy(), st)
	mem := audit.NewMemory()
	svc := appbuildtest.New(meta, appbuildtest.WithFS(fs, paths), appbuildtest.WithStore(st),
		appbuildtest.WithDeclarative(decl), appbuildtest.WithAudit(mem))
	app := newAppFromParts(&Config{}, nil, newFixture())
	rebindApp(app, fs, paths, svc)
	setTestACL(t, app, decl)

	deps := func() lua.WriteDeps {
		wd := app.luaWriteDeps()
		wd.ProjectRoot = root
		wd.Files = nil
		return wd
	}
	app.documents = newDocumentService(app.store, app.kv, root, script.NewEngine(), deps, nil)
	app.schema.Publish(&Schema{Cfg: &Config{
		Documents: map[string]dataentryconfig.DocumentConfig{
			"audit_report": {EntityType: "audit", Script: "docs/report.lua"},
			"summary":      {Script: "docs/summary.lua"},
			"gated":        {Script: "docs/summary.lua", Permission: "reports:view"},
			"elevated": {Script: "docs/summary.lua", Permission: "reports:view",
				AllowACLBypass: metamodel.ACLBypassRead},
		},
		Lists: map[string]dataentryconfig.List{
			"findings": {EntityType: "finding", Columns: []dataentryconfig.ListColumn{{Property: "title", Label: "Title"}}},
		},
		Actions: map[string]dataentryconfig.Action{},
	}, Meta: meta})
	var err error
	if app.export, err = newExportHandler(app); err != nil {
		t.Fatalf("newExportHandler: %v", err)
	}
	if err := app.SetSecurityConfig(SecurityConfig{BindAddress: "127.0.0.1:8080"}); err != nil {
		t.Fatal(err)
	}
	return &attachApp{app: app, decl: decl, store: st, audit: mem, root: root}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// runAction runs body as a fresh action script for user and returns the
// message the script returned.
func (a *attachApp) runAction(t *testing.T, user, body string) string {
	t.Helper()
	id := fmt.Sprintf("attach-%d", a.seq.Add(1))
	writeFile(t, filepath.Join(a.root, "actions", id+".lua"), body)
	s := a.app.State()
	cfg := *s.Cfg
	cfg.Actions = map[string]dataentryconfig.Action{id: {Label: id, Script: id + ".lua"}}
	a.app.schema.Publish(&Schema{Cfg: &cfg, Meta: s.Meta})
	rec := postAction(principalCtx(user), t, a.app, a.decl, id, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("action as %s: %d %s", user, rec.Code, rec.Body)
	}
	var resp script.ActionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return resp.Message
}

// attachCall is a Lua script that calls rela.attach_render with opts and
// returns "OK name size sha256 mime" or "ERR message".
func attachCall(opts string) string {
	return `local att, err = rela.attach_render{` + opts + `}
if not att then return { message = "ERR " .. err } end
return { message = "OK " .. att.name .. " " .. att.size .. " " .. att.sha256 .. " " .. att.mime }`
}

// stored returns the files on AUD-1's property and their bytes.
func (a *attachApp) stored(t *testing.T, property string) map[string][]byte {
	t.Helper()
	ctx := context.Background()
	infos, err := a.store.ListFamilyAttachments(ctx, "AUD-1")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]byte{}
	for _, info := range infos {
		if info.Property != property {
			continue
		}
		rc, err := a.store.ReadFamilyAttachment(ctx, "AUD-1", property, info.FileName)
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		if got := sha256Hex(data); info.SHA256 != got {
			t.Errorf("listed sha256 %s, bytes hash to %s", info.SHA256, got)
		}
		out[info.SHA256] = data
	}
	return out
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// okFields splits an "OK name size sha256 mime" message.
func okFields(t *testing.T, msg string) (name, size, sum, mime string) {
	t.Helper()
	f := strings.Fields(msg)
	if len(f) != 5 || f[0] != "OK" {
		t.Fatalf("attach_render failed: %s", msg)
	}
	return f[1], f[2], f[3], f[4]
}

func TestAttachRender_ActionAttachesPDFOfDocument(t *testing.T) {
	requireCp(t)
	a := newAttachApp(t)
	msg := a.runAction(t, "alice", attachCall(
		`entity = "AUD-1", property = "report", document = "audit_report", entry = "AUD-1", format = "pdf", filename = "report"`))
	name, size, sum, mimeType := okFields(t, msg)
	if name != "report.pdf" || mimeType != "application/pdf" {
		t.Errorf("name %q mime %q, want report.pdf application/pdf", name, mimeType)
	}

	files := a.stored(t, "report")
	data, ok := files[sum]
	if !ok {
		t.Fatalf("no stored file with the returned sha256 %s; stored %d files", sum, len(files))
	}
	if strconv.Itoa(len(data)) != size {
		t.Errorf("size %s, stored %d bytes", size, len(data))
	}
	body := string(data)
	for _, want := range []string{"%PDF-1.4", "# Report for Annual audit", "Weak passwords", "Open firewall"} {
		if !strings.Contains(body, want) {
			t.Errorf("stored file lacks %q:\n%s", want, body)
		}
	}

	// The API listing reports the same hash.
	rec := getEntityAs(principalCtx("alice"), t, a.app, a.decl, "audit", "audits", "AUD-1", "")
	if !strings.Contains(rec.Body.String(), `"sha256":"`+sum+`"`) {
		t.Errorf("entity response lacks the attachment hash %s: %s", sum, rec.Body)
	}

	var rendered []audit.Record
	for _, r := range a.audit.Records() {
		if r.Op == audit.OpAttachRender {
			rendered = append(rendered, r)
		}
	}
	if len(rendered) != 1 {
		t.Fatalf("attach-render audit records = %d, want 1", len(rendered))
	}
	for _, want := range []string{"sha256=" + sum, `document="audit_report"`, `entry="AUD-1"`, `format="pdf"`} {
		if !strings.Contains(rendered[0].Summary, want) {
			t.Errorf("audit summary %q lacks %q", rendered[0].Summary, want)
		}
	}
	if rendered[0].Principal.User != "alice" {
		t.Errorf("audit principal = %q, want alice", rendered[0].Principal.User)
	}
}

func TestAttachRender_HiddenRowsAreNotRendered(t *testing.T) {
	requireCp(t)
	for _, tc := range []struct {
		name, opts string
	}{
		{"document", `document = "audit_report", entry = "AUD-1"`},
		{"list", `list = "findings"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newAttachApp(t)
			msg := a.runAction(t, "carol", attachCall(
				`entity = "AUD-1", property = "report", format = "copy", filename = "out.txt", `+tc.opts))
			_, _, sum, _ := okFields(t, msg)
			body := string(a.stored(t, "report")[sum])
			for _, hidden := range []string{"Weak passwords", "Open firewall"} {
				if strings.Contains(body, hidden) {
					t.Errorf("carol cannot read findings, but the file contains %q:\n%s", hidden, body)
				}
			}

			// The same call as alice renders them, so the test can fail.
			msg = a.runAction(t, "alice", attachCall(
				`entity = "AUD-1", property = "report", format = "copy", filename = "out.txt", `+tc.opts))
			_, _, sum, _ = okFields(t, msg)
			if body := string(a.stored(t, "report")[sum]); !strings.Contains(body, "Weak passwords") {
				t.Errorf("alice's render lacks the findings:\n%s", body)
			}
		})
	}
}

func TestAttachRender_Refusals(t *testing.T) {
	requireCp(t)
	const target = `entity = "AUD-1", property = "report", format = "copy", filename = "x.txt", `
	for _, tc := range []struct {
		name, user, opts, want string
	}{
		{"no update grant", "dave", target + `list = "findings"`, "permission denied"},
		{"hidden entity", "mallory", target + `list = "findings"`, `entity "AUD-1" or its file property "report" not found`},
		{"missing entity", "alice", `entity = "AUD-9", property = "report", format = "copy", filename = "x.txt", list = "findings"`,
			`entity "AUD-9" or its file property "report" not found`},
		{"not a file property", "alice", `entity = "AUD-1", property = "title", format = "copy", filename = "x.txt", list = "findings"`,
			`file property "title" not found`},
		{"permission document", "carol", target + `document = "gated"`, `requires the "reports:view" permission`},
		{"elevated document", "alice", target + `document = "elevated"`, "elevated privileges"},
		{"unknown document", "alice", target + `document = "nope"`, `unknown document "nope"`},
		{"unknown list", "alice", target + `list = "nope"`, `unknown list "nope"`},
		{"unknown format", "alice", `entity = "AUD-1", property = "report", format = "docx", filename = "x", list = "findings"`,
			`unknown format "docx"`},
		{"extension mismatch", "alice", `entity = "AUD-1", property = "report", format = "pdf", filename = "x.txt", list = "findings"`,
			"does not match format"},
		{"entry on standalone", "alice", target + `document = "summary", entry = "AUD-1"`, "has no entity_type"},
		{"anchored without entry", "alice", target + `document = "audit_report"`, "is for entity_type"},
		{"rejected bytes", "alice", `entity = "AUD-1", property = "frozen", format = "pdf", filename = "x.pdf", list = "findings"`,
			"attachment rejected"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newAttachApp(t)
			msg := a.runAction(t, tc.user, attachCall(tc.opts))
			if !strings.HasPrefix(msg, "ERR ") || !strings.Contains(msg, tc.want) {
				t.Fatalf("message = %q, want an error containing %q", msg, tc.want)
			}
			if n := len(a.stored(t, "report")) + len(a.stored(t, "frozen")); n != 0 {
				t.Errorf("a refused call stored %d files", n)
			}
		})
	}
}

func TestAttachRender_WriteDenialIsAudited(t *testing.T) {
	requireCp(t)
	a := newAttachApp(t)
	a.runAction(t, "dave", attachCall(
		`entity = "AUD-1", property = "report", format = "copy", filename = "x.txt", list = "findings"`))
	for _, r := range a.audit.Records() {
		if r.Op == audit.OpDeniedWrite && strings.Contains(r.Summary, "op=attachment-write") {
			return
		}
	}
	t.Fatalf("no attachment-write denial recorded: %+v", a.audit.Records())
}

func TestAttachRender_MarkdownFormat(t *testing.T) {
	a := newAttachApp(t)
	// A .md file sniffs as text/plain, which the property accepts.
	name, _, sum, mimeType := okFields(t, a.runAction(t, "alice", attachCall(
		`entity = "AUD-1", property = "report", format = "md", filename = "summary", document = "summary"`)))
	if name != "summary.md" || mimeType != "text/markdown" {
		t.Errorf("name %q mime %q", name, mimeType)
	}
	if got := strings.TrimSpace(string(a.stored(t, "report")[sum])); got != "summary of findings" {
		t.Errorf("stored %q", got)
	}
}

// boundCtx is what a schedule or background automation hands a script: a
// principal and its bound ACL Request, but no read gate and no world.
func boundCtx(t *testing.T, a *attachApp, user string) context.Context {
	t.Helper()
	ctx := principalCtx(user)
	req, err := a.decl.ForIdentity(acltest.IdentityFor(t.Context(), t, a.decl, principal.From(ctx)))
	if err != nil {
		t.Fatal(err)
	}
	return acl.WithRequest(ctx, req)
}

// The scheduler and background automations call the renderer outside an
// HTTP request. Without the read gate it binds itself, every read below
// would see the allow-all gate.
func TestAttachRender_OutsideRequestIsGated(t *testing.T) {
	requireCp(t)
	a := newAttachApp(t)
	ar, err := newAttachRenderer(a.app)
	if err != nil {
		t.Fatal(err)
	}
	req := lua.AttachRenderRequest{
		Entity: "AUD-1", Property: "report", List: "findings", Format: "copy", Filename: "x.txt",
	}

	att, err := ar.AttachRender(boundCtx(t, a, "carol"), req)
	if err != nil {
		t.Fatal(err)
	}
	if body := string(a.stored(t, "report")[att.SHA256]); strings.Contains(body, "Weak passwords") {
		t.Errorf("carol's render outside a request shows hidden rows:\n%s", body)
	}

	if _, err := ar.AttachRender(boundCtx(t, a, "dave"), req); err == nil ||
		!strings.Contains(err.Error(), "permission denied") {

		t.Errorf("dave outside a request: err = %v, want permission denied", err)
	}
	gated := req
	gated.List, gated.Document = "", "gated"
	if _, err := ar.AttachRender(boundCtx(t, a, "carol"), gated); err == nil ||
		!strings.Contains(err.Error(), "reports:view") {

		t.Errorf("carol and a permission document outside a request: err = %v", err)
	}

	// No identity bound at all: refused, never read as anyone.
	if _, err := ar.AttachRender(principalCtx("alice"), req); err == nil ||
		!strings.Contains(err.Error(), "no identity") {

		t.Errorf("unbound ctx: err = %v, want a refusal", err)
	}
}

func TestAttachRender_ExpiredDeadlineStoresNothing(t *testing.T) {
	requireCp(t)
	a := newAttachApp(t)
	ar, err := newAttachRenderer(a.app)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithDeadline(boundCtx(t, a, "alice"), time.Now().Add(-time.Second))
	defer cancel()
	_, err = ar.AttachRender(ctx, lua.AttachRenderRequest{
		Entity: "AUD-1", Property: "report", Document: "summary", Format: "copy", Filename: "x.txt",
	})
	if err == nil {
		t.Fatal("an expired deadline still attached")
	}
	if n := len(a.stored(t, "report")); n != 0 {
		t.Errorf("stored %d files after the deadline", n)
	}
}

func TestNewAttachRenderer_RejectsMissingCollaborators(t *testing.T) {
	if _, err := newAttachRenderer(nil); err == nil {
		t.Error("nil app accepted")
	}
	if _, err := newAttachRenderer(&App{}); err == nil {
		t.Error("app without handlers accepted")
	}
}

func TestRenderFileName(t *testing.T) {
	for _, tc := range []struct {
		name, format, produces, want string
		wantErr                      bool
	}{
		{"report", "pdf", "application/pdf", "report.pdf", false},
		{"report.PDF", "pdf", "application/pdf", "report.PDF", false},
		{"report.txt", "pdf", "application/pdf", "", true},
		{"notes", "md", markdownMIME, "notes.md", false},
		{"notes.markdown", "md", markdownMIME, "notes.markdown", false},
		{"blob.bin", "odd", "application/x-unknown-type", "blob.bin", false},
		{"blob", "odd", "application/x-unknown-type", "blob.bin", false},
		{"blob.pdf", "odd", "application/x-unknown-type", "", true},
		{"audit-v1.2", "pdf", "application/pdf", "audit-v1.2.pdf", false},
		{"audit-v1.2.pdf", "pdf", "application/pdf", "audit-v1.2.pdf", false},
	} {
		got, err := renderFileName(tc.name, tc.format, tc.produces)
		if (err != nil) != tc.wantErr || got != tc.want {
			t.Errorf("renderFileName(%q, %q) = %q, %v; want %q, err %v", tc.name, tc.format, got, err, tc.want, tc.wantErr)
		}
	}
}
