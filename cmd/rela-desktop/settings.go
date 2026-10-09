package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/ai"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/hostconfig"
	"github.com/Sourcehaven-BV/rela/internal/mail"
	"github.com/Sourcehaven-BV/rela/internal/secrets"
	"github.com/Sourcehaven-BV/rela/internal/tokenstore"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"gopkg.in/yaml.v3"
)

// settingsPath is the route of the project settings window.
const settingsPath = "/_desktop/settings"

// settingsView is what the settings window shows. It carries secret NAMES
// only: a value goes into the keychain and never comes back to a page.
type settingsView struct {
	Project     string   `json:"project"`
	Secrets     []string `json:"secrets"`
	FileSecrets []string `json:"fileSecrets"`
	AI          string   `json:"ai"`
	Mail        string   `json:"mail"`
	// Untrusted is set when the keychain holds secrets for this document's
	// ID that this place may not read yet (see keychainSecrets).
	Untrusted bool `json:"untrusted"`
	// Connections lists the connections.yaml entries and whether each has
	// a token. Token values never come back to a page either.
	Connections []connectionView `json:"connections"`
	// ConnectionsError says why the project has no token store, when it
	// declares connections but cannot keep tokens.
	ConnectionsError string `json:"connectionsError,omitempty"`
	Error            string `json:"error,omitempty"`
}

// connectionView is one row of the Connections section.
type connectionView struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

// ProjectSettings is the bound service behind the settings window. It edits
// one project's secrets (in the keychain) and its AI and mail settings (in
// the project's state).
//
// Every page can call a bound service, including a document's own
// custom.js. So the project a call edits is never taken from the page: Go
// pins it to the settings window when it opens one (see open), and a call
// from any other window is refused. Approving secrets also goes through
// confirm, which page script cannot answer.
type ProjectSettings struct {
	d *Desktop
	// windows maps each open settings window to its project.
	//
	// Nil: rejected by newProjectSettings.
	windows *settingsWindows
	// confirm asks the user to approve message in a native dialog and
	// reports the answer.
	//
	// Nil: accepted — every approval is refused.
	confirm func(title, message, approve string) bool
}

// newProjectSettings returns the settings service for d.
func newProjectSettings(d *Desktop, confirm func(title, message, approve string) bool) (*ProjectSettings, error) {
	if d == nil {
		return nil, errors.New("project settings: nil desktop")
	}
	return &ProjectSettings{d: d, windows: &settingsWindows{byName: map[string]string{}}, confirm: confirm}, nil
}

// settingsWindows maps a settings window's name to the id of the project it
// edits.
type settingsWindows struct {
	mu     sync.Mutex
	byName map[string]string
}

func (w *settingsWindows) add(name, projectID string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.byName[name] = projectID
}

func (w *settingsWindows) remove(name string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.byName, name)
}

func (w *settingsWindows) project(name string) (string, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	id, ok := w.byName[name]
	return id, ok
}

// callerWindow is the name of the window a bound call came from, or "".
func callerWindow(ctx context.Context) string {
	if w, ok := ctx.Value(application.WindowKey).(interface{ Name() string }); ok && w != nil {
		return w.Name()
	}
	return ""
}

// errNoProject is returned when no project is open.
var errNoProject = errors.New("open a project first")

// errNotSettingsWindow refuses a call from a window that is not a settings
// window.
var errNotSettingsWindow = errors.New("settings can only be changed from the Project Settings window")

// open opens a settings window for the project with this id; "" means
// the active project. It returns an error message, or "".
// coverage-ignore-func: requires Wails runtime
func (s *ProjectSettings) open(id string) string {
	d := s.d
	if d.wails == nil {
		return "application not ready"
	}
	if id == "" {
		d.mu.RLock()
		app := d.app
		d.mu.RUnlock()
		if app == nil {
			return errNoProject.Error()
		}
		id = projectID(app.ProjectRoot())
	}
	name := fmt.Sprintf("settings-%d", windowSeq.Add(1))
	s.windows.add(name, id)
	application.InvokeAsync(func() {
		win := d.wails.Window.NewWithOptions(application.WebviewWindowOptions{
			Name:   name,
			Title:  "Project Settings",
			URL:    settingsPath,
			Width:  defaultSecondaryWidth,
			Height: defaultSecondaryHeight,
			Mac:    macWindow(),
		})
		win.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) { s.windows.remove(name) })
		d.menu.trackWindow(win, id)
		win.Show()
	})
	return ""
}

// settingsTarget is the project a settings call edits.
type settingsTarget struct {
	host *desktopHost
	name string
	// tokens manages the project's connector tokens; nil with tokensErr
	// saying why when it has none.
	tokens    *tokenstore.Broker
	tokensErr error
	// active is set when the project is the one at the bare root, the only
	// one SaveFile can reopen.
	active bool
}

// target resolves the project pinned to the calling window.
func (s *ProjectSettings) target(ctx context.Context) (settingsTarget, error) {
	name := callerWindow(ctx)
	if name == "" || s.windows == nil {
		return settingsTarget{}, errNotSettingsWindow
	}
	id, ok := s.windows.project(name)
	if !ok {
		return settingsTarget{}, errNotSettingsWindow
	}
	d := s.d
	p := d.registry.get(id)
	if p == nil || p.svc == nil || p.app == nil || p.svc.Paths() == nil {
		return settingsTarget{}, errors.New("that project is no longer open")
	}
	if d.keychain == nil {
		return settingsTarget{}, errors.New("the keychain is not available")
	}
	host, err := newDesktopHost(ctx, p.svc.Paths().CacheDir, p.svc.State(), d.keychain)
	if err != nil {
		return settingsTarget{}, err
	}
	d.mu.RLock()
	active := d.app == p.app
	d.mu.RUnlock()
	t := settingsTarget{host: host, name: p.app.ProjectName(), active: active}
	t.tokens, t.tokensErr = appbuild.Tokens(p.svc)
	return t, nil
}

// Load returns a project's settings.
func (s *ProjectSettings) Load(ctx context.Context) settingsView {
	t, err := s.target(ctx)
	if err != nil {
		return settingsView{Error: err.Error()}
	}
	host := t.host
	view := settingsView{Project: t.name}
	fromKeychain, err := host.secrets.all(host.docID)
	if err != nil {
		view.Error = err.Error()
	}
	view.Secrets = slices.Sorted(maps.Keys(withoutTokenItems(fromKeychain)))
	view.Connections, view.ConnectionsError = connectionRows(ctx, t)
	if _, trusted, err := host.secrets.forPlace(host.docID, host.Path()); err == nil {
		view.Untrusted = !trusted
	}
	if fromFile, err := host.dir.Secrets(""); err == nil {
		view.FileSecrets = slices.Sorted(maps.Keys(fromFile))
	} else if !errors.Is(err, secrets.ErrNotFound) && view.Error == "" {
		view.Error = err.Error()
	}
	view.AI = hostFileText(host, hostconfig.AIFile)
	view.Mail = hostFileText(host, hostconfig.MailFile)
	return view
}

// SetSecret stores a secret in the keychain. It returns "" or an error message.
func (s *ProjectSettings) SetSecret(ctx context.Context, name, value string) string {
	t, err := s.target(ctx)
	if err == nil {
		err = t.host.secrets.set(t.host.docID, t.host.Path(), strings.TrimSpace(name), value)
	}
	return errText(err)
}

// connectionRows lists the declared connections with their token state.
func connectionRows(ctx context.Context, t settingsTarget) (rows []connectionView, errMsg string) {
	if t.tokens == nil {
		return nil, errText(t.tokensErr)
	}
	names := slices.Sorted(maps.Keys(t.tokens.Connections()))
	rows = make([]connectionView, 0, len(names))
	for _, n := range names {
		st, err := t.tokens.Status(ctx, string(n))
		row := connectionView{Name: string(n)}
		switch {
		case err != nil:
			row.State = err.Error()
		case st.Err != nil:
			row.State = "Cannot read the token: " + st.Err.Error()
		case !st.Stored:
			row.State = "No token"
		case !st.NeedsConsentAt.IsZero():
			row.State = "Needs consent: run the consent flow and paste the new token"
		default:
			row.State = "Connected"
		}
		rows = append(rows, row)
	}
	return rows, ""
}

// SetToken stores a connection's token: the refresh token, or the
// provider's JSON token response, as `rela token set` reads it. It returns
// "" or an error message.
func (s *ProjectSettings) SetToken(ctx context.Context, name, input string) string {
	t, err := s.target(ctx)
	if err != nil {
		return err.Error()
	}
	if t.tokens == nil {
		return errText(t.tokensErr)
	}
	tok, err := tokenstore.ParseInput([]byte(input), time.Now())
	if err == nil {
		err = t.tokens.Set(ctx, name, tok)
	}
	return errText(err)
}

// DeleteToken removes a connection's token. It returns "" or an error
// message.
func (s *ProjectSettings) DeleteToken(ctx context.Context, name string) string {
	t, err := s.target(ctx)
	if err != nil {
		return err.Error()
	}
	if t.tokens == nil {
		return errText(t.tokensErr)
	}
	return errText(t.tokens.Delete(ctx, name))
}

// errTrustDeclined is returned when the user does not approve.
var errTrustDeclined = errors.New("not allowed")

// TrustSecrets lets a project's scripts read the keychain secrets stored for
// its document ID, once the user approves in a native dialog. It returns ""
// or an error message.
func (s *ProjectSettings) TrustSecrets(ctx context.Context) string {
	t, err := s.target(ctx)
	if err != nil {
		return err.Error()
	}
	msg := fmt.Sprintf("Allow %q to use the secrets stored for this document?\n\n"+
		"Only allow a document you moved or copied yourself. A document from "+
		"someone else can carry the same ID, and its scripts would then read "+
		"your secrets.", t.name)
	if s.confirm == nil || !s.confirm("Allow Secrets", msg, "Allow") {
		return errTrustDeclined.Error()
	}
	return errText(t.host.secrets.trust(t.host.docID, t.host.Path()))
}

// DeleteSecret removes a secret from the keychain. It returns "" or an
// error message.
func (s *ProjectSettings) DeleteSecret(ctx context.Context, name string) string {
	t, err := s.target(ctx)
	if err == nil {
		err = t.host.secrets.remove(t.host.docID, t.host.Path(), name)
	}
	return errText(err)
}

// SaveFile stores ai.yaml or mail.yaml in the project's state after checking
// it parses; empty content removes it. Saving mail settings reopens the
// project, because its mail sender is built once when the project opens.
// It returns "" or an error message.
func (s *ProjectSettings) SaveFile(ctx context.Context, name, content string) string {
	t, err := s.target(ctx)
	if err != nil {
		return err.Error()
	}
	host := t.host
	if err = hostconfig.CheckName(name); err != nil {
		return err.Error()
	}
	key := hostFileKeyPrefix + name
	if strings.TrimSpace(content) == "" {
		err = host.kv.Delete(ctx, key)
	} else if err = checkHostFile(name, []byte(content)); err == nil {
		err = host.kv.Put(ctx, key, []byte(content))
	}
	if err != nil {
		return err.Error()
	}
	if name == hostconfig.MailFile {
		if !t.active {
			return "Saved. Reopen the project to use the new mail settings."
		}
		s.d.mu.RLock()
		path := s.d.activePath
		s.d.mu.RUnlock()
		//nolint:contextcheck // the reopened project outlives this call, so it must not inherit its context
		if msg := s.d.loadProject(path, false); msg != "" && msg != "needs_setup" {
			return "Saved, but reopening the project failed: " + msg
		}
	}
	return ""
}

// checkHostFile parses content as the named file would be parsed on use.
func checkHostFile(name string, content []byte) error {
	if name == hostconfig.AIFile {
		// Strict, so a key typed in as api_key is refused rather than stored
		// in the document, where it would travel with the file.
		dec := yaml.NewDecoder(bytes.NewReader(content))
		dec.KnownFields(true)
		var cfg ai.Config
		if err := dec.Decode(&cfg); err != nil && !errors.Is(err, io.EOF) {
			return fmt.Errorf("%s: %w (put the API key in the secret %s)", name, err, ai.SecretKey)
		}
		_, err := ai.ParseConfig(content, name)
		return err
	}
	_, err := mail.ParseConfig(content, name)
	return err
}

// hostFileText returns a host config file's content, or "" when absent or
// unreadable.
func hostFileText(host *desktopHost, name string) string {
	data, err := host.File(name)
	if err != nil {
		return ""
	}
	return string(data)
}

// errText is err's message, or "" for nil.
func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// serveSettingsPage writes the settings window. A static page: it reads
// everything through the ProjectSettings service.
func serveSettingsPage(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(settingsPage))
}

const settingsPage = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Project Settings</title>
<style>
  :root { color-scheme: light dark; font: 13px -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
  body { margin: 0; padding: 28px 24px 24px; }
  h1 { font-size: 18px; margin: 0 0 4px; }
  h2 { font-size: 14px; margin: 24px 0 6px; }
  p.hint { color: GrayText; margin: 0 0 8px; }
  table { border-collapse: collapse; width: 100%; }
  td { padding: 4px 0; border-bottom: 1px solid color-mix(in srgb, CanvasText 12%, transparent); }
  td.actions { text-align: right; }
  input, textarea, button { font: inherit; }
  input[type=text], input[type=password] { width: 100%; box-sizing: border-box; }
  textarea { width: 100%; box-sizing: border-box; min-height: 110px; font-family: ui-monospace, Menlo, monospace; }
  .row { display: flex; gap: 8px; margin-top: 8px; }
  .row > * { flex: 1; }
  .row > button { flex: none; }
  [hidden] { display: none !important; }
  #status { min-height: 1.4em; margin-top: 12px; }
  #status.error { color: #c0392b; }
</style>
</head>
<body>
<h1>Project Settings</h1>
<p class="hint" id="project"></p>

<h2>Secrets</h2>
<p class="hint">Stored in this computer's keychain, not in the project. Scripts read them with rela.secret().</p>
<div id="untrusted" hidden>
  <p>This document's scripts cannot read the secrets below yet. They were stored for a document with the
  same ID at another location. If you did not copy or move this document yourself, it may come from
  someone else, and its scripts would read your secrets.</p>
  <div class="row"><span></span><button id="trust">Allow This Document to Use Them</button></div>
</div>
<table id="secrets"></table>
<p class="hint" id="file-secrets"></p>
<form id="add-secret" class="row">
  <input type="text" id="secret-name" placeholder="Name" autocomplete="off" spellcheck="false">
  <input type="password" id="secret-value" placeholder="Value" autocomplete="off">
  <button type="submit">Save Secret</button>
</form>

<h2>Connections</h2>
<p class="hint">OAuth connections from connections.yaml. Paste the refresh token the consent flow printed,
or its whole JSON token response. Stored in the keychain; scripts get only short-lived access tokens.</p>
<p class="hint" id="connections-error"></p>
<table id="connections"></table>
<form id="add-token" class="row">
  <select id="token-name"></select>
  <input type="password" id="token-value" placeholder="Refresh token or JSON" autocomplete="off">
  <button type="submit">Save Token</button>
</form>

<h2>AI</h2>
<p class="hint">ai.yaml: provider, base_url, model. Put the API key in a secret named ai_api_key.</p>
<textarea id="ai" spellcheck="false"></textarea>
<div class="row"><span></span><button id="save-ai">Save AI Settings</button></div>

<h2>Mail</h2>
<p class="hint">mail.yaml. Put the SMTP password in a secret named smtp_password. Saving reopens the project.</p>
<textarea id="mail" spellcheck="false"></textarea>
<div class="row"><span></span><button id="save-mail">Save Mail Settings</button></div>

<div id="status"></div>

<script type="module">
import "/wails/runtime.js";

const call = (method, ...args) => window.wails.Call.ByName("main.ProjectSettings." + method, ...args);
const $ = (id) => document.getElementById(id);

function status(message, isError) {
  $("status").textContent = message || "";
  $("status").className = isError ? "error" : "";
}

async function load() {
  const view = await call("Load");
  $("project").textContent = view.project ? "For " + view.project + "." : "";
  const table = $("secrets");
  table.replaceChildren();
  for (const name of view.secrets || []) {
    const row = table.insertRow();
    row.insertCell().textContent = name;
    const actions = row.insertCell();
    actions.className = "actions";
    if (view.untrusted) continue; // removing is refused until allowed
    const remove = document.createElement("button");
    remove.textContent = "Remove";
    remove.onclick = async () => done(await call("DeleteSecret", name), "Removed " + name + ".");
    actions.append(remove);
  }
  if (!(view.secrets || []).length) table.insertRow().insertCell().textContent = "No secrets in the keychain.";
  const fromFile = view.fileSecrets || [];
  $("file-secrets").textContent = fromFile.length
    ? "Also read from .rela/secrets.yaml: " + fromFile.join(", ") + ". A keychain secret with the same name wins."
    : "";
  $("untrusted").hidden = !view.untrusted;
  const conns = $("connections");
  conns.replaceChildren();
  const select = $("token-name");
  select.replaceChildren();
  for (const c of view.connections || []) {
    const row = conns.insertRow();
    row.insertCell().textContent = c.name;
    row.insertCell().textContent = c.state;
    const actions = row.insertCell();
    actions.className = "actions";
    const remove = document.createElement("button");
    remove.textContent = "Remove Token";
    remove.onclick = async () => done(await call("DeleteToken", c.name), "Removed the token for " + c.name + ".");
    actions.append(remove);
    select.append(new Option(c.name, c.name));
  }
  if (!(view.connections || []).length) conns.insertRow().insertCell().textContent = "No connections declared.";
  $("connections-error").textContent = view.connectionsError || "";
  $("add-token").hidden = !(view.connections || []).length;
  $("ai").value = view.ai || "";
  $("mail").value = view.mail || "";
  if (view.error) status(view.error, true);
}

async function done(err, message) {
  if (err) { status(err, true); return; }
  status(message, false);
  await load();
}

$("trust").onclick = async () => done(await call("TrustSecrets"), "This document can use the secrets now.");

$("add-secret").onsubmit = async (event) => {
  event.preventDefault();
  const name = $("secret-name").value.trim();
  const err = await call("SetSecret", name, $("secret-value").value);
  if (!err) { $("secret-name").value = ""; $("secret-value").value = ""; }
  await done(err, "Saved " + name + ".");
};
$("add-token").onsubmit = async (event) => {
  event.preventDefault();
  const name = $("token-name").value;
  const err = await call("SetToken", name, $("token-value").value);
  if (!err) $("token-value").value = "";
  await done(err, "Saved the token for " + name + ".");
};
$("save-ai").onclick = async () => done(await call("SaveFile", "ai.yaml", $("ai").value), "Saved AI settings.");
$("save-mail").onclick = async () => done(await call("SaveFile", "mail.yaml", $("mail").value), "Saved mail settings.");

load().catch((err) => status(String(err), true));
</script>
</body>
</html>
`
